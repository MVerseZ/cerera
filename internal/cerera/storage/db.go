// source code partially copied from
// https://github.com/recoilme/pudge
// edited by gnupunk

package storage

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var (
	dbs struct {
		sync.RWMutex
		dbs map[string]*Db
	}
	ErrKeyNotFound = errors.New("error: key not found")
	mutex          = &sync.RWMutex{}
)

// Db represent database
type Db struct {
	sync.RWMutex
	name         string
	fk           *os.File
	fv           *os.File
	keys         [][]byte
	vals         map[string]*Cmd
	cancelSyncer context.CancelFunc
	storemode    int
}

// Cmd represent keys and vals addresses
type Cmd struct {
	Seek    uint32
	Size    uint32
	KeySeek uint32
	Val     []byte
}

// Config fo db
// Default FileMode = 0644
// Default DirMode = 0755
// Default SyncInterval = 0 sec, 0 - disable sync (os will sync, typically 30 sec or so)
// If StroreMode==2 && file == "" - pure inmemory mode
type Config struct {
	FileMode     int // 0644
	DirMode      int // 0755
	SyncInterval int // in seconds
	StoreMode    int // 0 - file first, 2 - memory first(with persist on close), 2 - with empty file - memory without persist
}

func init() {
	dbs.dbs = make(map[string]*Db)
}

func newDb(f string, cfg *Config) (*Db, error) {
	var err error
	// create
	db := new(Db)
	db.Lock()
	defer db.Unlock()
	// init
	db.name = f
	db.keys = make([][]byte, 0)
	db.vals = make(map[string]*Cmd)
	db.storemode = cfg.StoreMode

	// Apply default values
	if cfg.FileMode == 0 {
		cfg.FileMode = DefaultConfig.FileMode
	}
	if cfg.DirMode == 0 {
		cfg.DirMode = DefaultConfig.DirMode
	}
	if db.storemode == 2 && db.name == "" {
		return db, nil
	}
	_, err = os.Stat(f)
	if err != nil {
		// file not exists - create dirs if any
		if os.IsNotExist(err) {
			if filepath.Dir(f) != "." {
				err = os.MkdirAll(filepath.Dir(f), os.FileMode(cfg.DirMode))
				if err != nil {
					return nil, err
				}
			}
		} else {
			return nil, err
		}
	}
	db.fv, err = os.OpenFile(f, os.O_CREATE|os.O_RDWR, os.FileMode(cfg.FileMode))
	if err != nil {
		return nil, err
	}
	db.fk, err = os.OpenFile(f+".idx", os.O_CREATE|os.O_RDWR, os.FileMode(cfg.FileMode))
	if err != nil {
		return nil, err
	}
	//read keys
	buf := new(bytes.Buffer)
	b, err := ioutil.ReadAll(db.fk)
	if err != nil {
		return nil, err
	}
	buf.Write(b)
	var readSeek uint32
	for buf.Len() > 0 {
		_ = uint8(buf.Next(1)[0]) //format version
		t := uint8(buf.Next(1)[0])
		seek := binary.BigEndian.Uint32(buf.Next(4))
		size := binary.BigEndian.Uint32(buf.Next(4))
		_ = buf.Next(4) //time
		sizeKey := int(binary.BigEndian.Uint16(buf.Next(2)))
		key := buf.Next(sizeKey)
		strkey := string(key)
		cmd := &Cmd{
			Seek:    seek,
			Size:    size,
			KeySeek: readSeek,
		}
		if db.storemode == 2 {
			cmd.Val = make([]byte, size)
			db.fv.ReadAt(cmd.Val, int64(seek))
		}
		readSeek += uint32(16 + sizeKey)
		switch t {
		case 0:
			if _, exists := db.vals[strkey]; !exists {
				//write new key at keys store
				db.appendKey(key)
			}
			db.vals[strkey] = cmd
		case 1:
			delete(db.vals, strkey)
			db.deleteFromKeys(key)
		}
	}

	if cfg.SyncInterval > 0 {
		db.backgroundManager(cfg.SyncInterval)
	}
	return db, err
}

// backgroundManager runs continuously in the background and performs various
// operations such as syncing to disk.
func (db *Db) backgroundManager(interval int) {
	ctx, cancel := context.WithCancel(context.Background())
	db.cancelSyncer = cancel
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				db.Lock()
				db.fk.Sync()
				db.fv.Sync()
				db.Unlock()
				time.Sleep(time.Duration(interval) * time.Second)
			}
		}
	}()
}

// appendKey insert key in slice
func (db *Db) appendKey(b []byte) {
	//log.Println("append")
	db.keys = append(db.keys, b)
}

// deleteFromKeys delete key from slice keys
func (db *Db) deleteFromKeys(b []byte) {
	found := db.found(b)
	if found < len(db.keys) {
		if bytes.Equal(db.keys[found], b) {
			db.keys = append(db.keys[:found], db.keys[found+1:]...)
		}
	}
}

func (db *Db) sort() {
	if !sort.SliceIsSorted(db.keys, db.lessBinary) {
		//log.Println("sort")
		sort.Slice(db.keys, db.lessBinary)
	}
}

func (db *Db) lessBinary(i, j int) bool {
	return bytes.Compare(db.keys[i], db.keys[j]) <= 0
}

// found return binary search result with sort order
func (db *Db) found(b []byte) int {
	db.sort()
	return sort.Search(len(db.keys), func(i int) bool {
		return bytes.Compare(db.keys[i], b) >= 0
	})
}

// KeyToBinary return key in bytes
func KeyToBinary(v interface{}) ([]byte, error) {
	var err error

	switch v.(type) {
	case []byte:
		return v.([]byte), nil
	case bool, float32, float64, complex64, complex128, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		buf := new(bytes.Buffer)
		err = binary.Write(buf, binary.BigEndian, v)
		return buf.Bytes(), err
	case int:
		val := uint64(v.(int))
		p := make([]byte, 8)
		p[0] = byte(val >> 56)
		p[1] = byte(val >> 48)
		p[2] = byte(val >> 40)
		p[3] = byte(val >> 32)
		p[4] = byte(val >> 24)
		p[5] = byte(val >> 16)
		p[6] = byte(val >> 8)
		p[7] = byte(val)
		return p, err
	case string:
		return []byte(v.(string)), nil
	default:
		buf := new(bytes.Buffer)
		err = gob.NewEncoder(buf).Encode(v)
		return buf.Bytes(), err
	}
}

// ValToBinary return value in bytes
func ValToBinary(v interface{}) ([]byte, error) {
	var err error
	switch v.(type) {
	case []byte:
		return v.([]byte), nil
	default:
		buf := new(bytes.Buffer)
		err = gob.NewEncoder(buf).Encode(v)
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), err
	}
}

func writeKeyVal(fk, fv *os.File, readKey, writeVal []byte, exists bool, oldCmd *Cmd) (cmd *Cmd, err error) {

	var seek, newSeek int64
	cmd = &Cmd{Size: uint32(len(writeVal))}
	if exists {
		// key exists
		cmd.Seek = oldCmd.Seek
		cmd.KeySeek = oldCmd.KeySeek
		if oldCmd.Size >= uint32(len(writeVal)) {
			//write at old seek new value
			_, _, err = writeAtPos(fv, writeVal, int64(oldCmd.Seek))
		} else {
			//write at new seek (at the end of file)
			seek, _, err = writeAtPos(fv, writeVal, int64(-1))
			cmd.Seek = uint32(seek)
		}
		if err == nil {
			// if no error - store key at KeySeek
			newSeek, err = writeKey(fk, 0, cmd.Seek, cmd.Size, []byte(readKey), int64(cmd.KeySeek))
			cmd.KeySeek = uint32(newSeek)
		}
	} else {
		// new key
		// write value at the end of file
		seek, _, err = writeAtPos(fv, writeVal, int64(-1))
		cmd.Seek = uint32(seek)
		if err == nil {
			newSeek, err = writeKey(fk, 0, cmd.Seek, cmd.Size, []byte(readKey), -1)
			cmd.KeySeek = uint32(newSeek)
		}
	}
	return cmd, err
}

// if pos<0 store at the end of file
func writeAtPos(f *os.File, b []byte, pos int64) (seek int64, n int, err error) {
	seek = pos
	if pos < 0 {
		seek, err = f.Seek(0, 2)
		if err != nil {
			return seek, 0, err
		}
	}
	n, err = f.WriteAt(b, seek)
	if err != nil {
		return seek, n, err
	}
	return seek, n, err
}

// writeKey create buffer and store key with val address and size
func writeKey(fk *os.File, t uint8, seek, size uint32, key []byte, keySeek int64) (newSeek int64, err error) {
	//get buf from pool
	buf := new(bytes.Buffer)
	buf.Reset()
	buf.Grow(16 + len(key))

	//encode
	binary.Write(buf, binary.BigEndian, uint8(0))                  //1byte version
	binary.Write(buf, binary.BigEndian, t)                         //1byte command code(0-set,1-delete)
	binary.Write(buf, binary.BigEndian, seek)                      //4byte seek
	binary.Write(buf, binary.BigEndian, size)                      //4byte size
	binary.Write(buf, binary.BigEndian, uint32(time.Now().Unix())) //4byte timestamp
	binary.Write(buf, binary.BigEndian, uint16(len(key)))          //2byte key size
	buf.Write(key)                                                 //key

	if keySeek < 0 {
		newSeek, _, err = writeAtPos(fk, buf.Bytes(), int64(-1))
	} else {
		newSeek, _, err = writeAtPos(fk, buf.Bytes(), int64(keySeek))
	}

	return newSeek, err
}

// findKey return index of first key in ascending mode
// findKey return index of last key in descending mode
// findKey return 0 or len-1 in case of nil key
func (db *Db) findKey(key interface{}, asc bool) (int, error) {
	if key == nil {
		db.sort()
		if asc {
			return 0, ErrKeyNotFound
		}
		return len(db.keys) - 1, ErrKeyNotFound
	}
	k, err := KeyToBinary(key)
	if err != nil {
		return -1, err
	}
	found := db.found(k)
	//log.Println("found", found)
	// check found
	if found >= len(db.keys) {
		return -1, ErrKeyNotFound
	}
	if !bytes.Equal(db.keys[found], k) {
		return -1, ErrKeyNotFound
	}
	return found, nil
}

// startFrom return is a start from b in binary
func startFrom(a, b []byte) bool {
	if a == nil || b == nil {
		return false
	}
	if len(a) < len(b) {
		return false
	}
	return bytes.Compare(a[:len(b)], b) == 0
}

func (db *Db) foundPref(b []byte, asc bool) int {
	db.sort()
	if asc {
		return sort.Search(len(db.keys), func(i int) bool {
			return bytes.Compare(db.keys[i], b) >= 0
		})
	}
	var j int
	for j = len(db.keys) - 1; j >= 0; j-- {
		if startFrom(db.keys[j], b) {
			break
		}
	}
	return j
}

func checkInterval(find, limit, offset, excludeFrom, len int, asc bool) (int, int) {
	end := 0
	start := find

	if asc {
		start += (offset + excludeFrom)
		if limit == 0 {
			end = len - excludeFrom
		} else {
			end = (start + limit - 1)
		}
	} else {
		start -= (offset + excludeFrom)
		if limit == 0 {
			end = 0
		} else {
			end = start - limit + 1
		}
	}

	if end < 0 {
		end = 0
	}
	if end >= len {
		end = len - 1
	}

	return start, end
}

/*
	CONFIG AND API METHODS
*/
// DefaultConfig is default config
var DefaultConfig = &Config{
	FileMode:     0644,
	DirMode:      0755,
	SyncInterval: 0,
	StoreMode:    0}

// Open return db object if it opened.
// Create new db if not exist.
// Read db to obj if exist.
// Or error if any.
// Default Config (if nil): &Config{FileMode: 0644, DirMode: 0755, SyncInterval: 0}
func Open(f string, cfg *Config) (*Db, error) {
	if cfg == nil {
		cfg = DefaultConfig
	}
	dbs.RLock()
	db, ok := dbs.dbs[f]
	if ok {
		dbs.RUnlock()
		return db, nil
	}
	dbs.RUnlock()
	dbs.Lock()
	db, err := newDb(f, cfg)
	if err == nil {
		dbs.dbs[f] = db
	}
	dbs.Unlock()
	return db, err
}

// Set store any key value to db
func (db *Db) Set(key, value interface{}) error {
	db.Lock()
	defer db.Unlock()
	k, err := KeyToBinary(key)
	if err != nil {
		return err
	}
	v, err := ValToBinary(value)
	if err != nil {
		return err
	}
	oldCmd, exists := db.vals[string(k)]
	if db.storemode == 2 {
		cmd := &Cmd{}
		cmd.Size = uint32(len(v))
		cmd.Val = make([]byte, len(v))
		copy(cmd.Val, v)
		db.vals[string(k)] = cmd
	} else {
		cmd, err := writeKeyVal(db.fk, db.fv, k, v, exists, oldCmd)
		if err != nil {
			return err
		}
		db.vals[string(k)] = cmd
	}
	if !exists {
		db.appendKey(k)
	}

	return err
}

// Get return value by key
// Return error if any.
func (db *Db) Get(key, value interface{}) error {
	db.RLock()
	defer db.RUnlock()
	k, err := KeyToBinary(key)
	if err != nil {
		return err
	}
	if val, ok := db.vals[string(k)]; ok {
		switch value.(type) {
		case *[]byte:
			b := make([]byte, val.Size)
			if db.storemode == 2 {
				copy(b, val.Val)
			} else {
				_, err := db.fv.ReadAt(b, int64(val.Seek))
				if err != nil {
					return err
				}
			}
			*value.(*[]byte) = b
			return nil
		default:

			buf := new(bytes.Buffer)
			b := make([]byte, val.Size)
			if db.storemode == 2 {
				copy(b, val.Val)
			} else {
				_, err := db.fv.ReadAt(b, int64(val.Seek))
				if err != nil {
					return err
				}
			}
			buf.Write(b)
			err = gob.NewDecoder(buf).Decode(value)
			return err
		}
	}
	return ErrKeyNotFound
}

// Close - sync & close files.
// Return error if any.
func (db *Db) Close() error {
	if db.cancelSyncer != nil {
		db.cancelSyncer()
	}
	db.Lock()
	defer db.Unlock()

	if db.storemode == 2 && db.name != "" {
		db.sort()
		keys := make([][]byte, len(db.keys))

		copy(keys, db.keys)

		db.storemode = 0
		for _, k := range keys {
			if val, ok := db.vals[string(k)]; ok {
				writeKeyVal(db.fk, db.fv, k, val.Val, false, nil)
			}
		}
	}
	if db.fk != nil {
		err := db.fk.Sync()
		if err != nil {
			return err
		}
		err = db.fk.Close()
		if err != nil {
			return err
		}
	}
	if db.fv != nil {
		err := db.fv.Sync()
		if err != nil {
			return err
		}

		err = db.fv.Close()
		if err != nil {
			return err
		}
	}

	dbs.Lock()
	delete(dbs.dbs, db.name)
	dbs.Unlock()
	return nil
}

// CloseAll - close all opened Db
func CloseAll() (err error) {
	dbs.Lock()
	stores := dbs.dbs
	dbs.Unlock()
	for _, db := range stores {
		err = db.Close()
		if err != nil {
			break
		}
	}

	return err
}

// DeleteFile close and delete file
func (db *Db) DeleteFile() error {
	return DeleteFile(db.name)
}

// DeleteFile close db and delete file
func DeleteFile(file string) error {
	if file == "" {
		return nil
	}
	dbs.Lock()
	db, ok := dbs.dbs[file]
	if ok {
		dbs.Unlock()
		err := db.Close()
		if err != nil {
			return err
		}
	} else {
		dbs.Unlock()
	}

	err := os.Remove(file)
	if err != nil {
		return err
	}
	err = os.Remove(file + ".idx")
	return err
}

// Has return true if key exists.
// Return error if any.
func (db *Db) Has(key interface{}) (bool, error) {
	db.RLock()
	defer db.RUnlock()
	k, err := KeyToBinary(key)
	if err != nil {
		return false, err
	}
	_, has := db.vals[string(k)]
	return has, nil
}

// FileSize returns the total size of the disk storage used by the DB.
func (db *Db) FileSize() (int64, error) {
	db.RLock()
	defer db.RUnlock()
	var err error
	is, err := db.fk.Stat()
	if err != nil {
		return -1, err
	}
	ds, err := db.fv.Stat()
	if err != nil {
		return -1, err
	}
	return is.Size() + ds.Size(), nil
}

// Count returns the number of items in the Db.
func (db *Db) Count() (int, error) {
	db.RLock()
	defer db.RUnlock()
	return len(db.keys), nil
}

// Delete remove key
// Returns error if key not found
func (db *Db) Delete(key interface{}) error {
	db.Lock()
	defer db.Unlock()
	k, err := KeyToBinary(key)
	if err != nil {
		return err
	}
	if _, ok := db.vals[string(k)]; ok {
		delete(db.vals, string(k))
		db.deleteFromKeys(k)
		writeKey(db.fk, 1, 0, 0, k, -1)
		return nil
	}
	return ErrKeyNotFound
}

// KeysByPrefix return keys with prefix
// in ascending  or descending order (false - descending,true - ascending)
// if limit == 0 return all keys
// if offset > 0 - skip offset records
// If from not nil - return keys after from (from not included)
func (db *Db) KeysByPrefix(prefix []byte, limit, offset int, asc bool) ([][]byte, error) {
	//log.Println("KeysByPrefix")
	db.RLock()
	defer db.RUnlock()
	// resulting array
	arr := make([][]byte, 0, 0)
	found := db.foundPref(prefix, asc)
	if found < 0 || found >= len(db.keys) || !startFrom(db.keys[found], prefix) {
		//not found
		return arr, ErrKeyNotFound
	}

	start, end := checkInterval(found, limit, offset, 0, len(db.keys), asc)

	if start < 0 || start >= len(db.keys) {
		return arr, nil
	}

	if asc {
		for i := start; i <= end; i++ {
			if !startFrom(db.keys[i], prefix) {
				break
			}
			arr = append(arr, db.keys[i])
		}
	} else {
		for i := start; i >= end; i-- {
			if !startFrom(db.keys[i], prefix) {
				break
			}
			arr = append(arr, db.keys[i])
		}
	}
	return arr, nil
}

// Keys return keys in ascending  or descending order (false - descending,true - ascending)
// if limit == 0 return all keys
// if offset > 0 - skip offset records
// If from not nil - return keys after from (from not included)
func (db *Db) Keys(from interface{}, limit, offset int, asc bool) ([][]byte, error) {
	arr := make([][]byte, 0, 0)
	excludeFrom := 0
	if from != nil {
		excludeFrom = 1

		k, err := KeyToBinary(from)
		if err != nil {
			return arr, err
		}
		if len(k) > 1 && bytes.Equal(k[len(k)-1:], []byte("*")) {
			byteOrStr := false
			switch from.(type) {
			case []byte:
				byteOrStr = true
			case string:
				byteOrStr = true
			}
			if byteOrStr {
				prefix := make([]byte, len(k)-1)
				copy(prefix, k)
				return db.KeysByPrefix(prefix, limit, offset, asc)
			}
		}
	}
	db.RLock()
	defer db.RUnlock()
	find, err := db.findKey(from, asc)
	if from != nil && err != nil {
		return nil, err
	}
	start, end := checkInterval(find, limit, offset, excludeFrom, len(db.keys), asc)
	if start < 0 || start >= len(db.keys) {
		return arr, nil
	}

	if asc {
		for i := start; i <= end; i++ {
			arr = append(arr, db.keys[i])
		}
	} else {
		for i := start; i >= end; i-- {
			arr = append(arr, db.keys[i])
		}
	}
	return arr, nil
}

// Counter return int64 incremented on incr
func (db *Db) Counter(key interface{}, incr int) (int64, error) {
	mutex.Lock()
	var counter int64
	err := db.Get(key, &counter)
	if err != nil && err != ErrKeyNotFound {
		return -1, err
	}
	//mutex.Lock()
	counter = counter + int64(incr)
	//mutex.Unlock()
	err = db.Set(key, counter)
	mutex.Unlock()
	return counter, err
}

// Set store any key value to db with opening if needed
func Set(f string, key, value interface{}) error {
	db, err := Open(f, nil)
	if err != nil {
		return err
	}
	return db.Set(key, value)
}

// Sets store vals and keys
// Use it for mass insertion
// every pair must contain key and value
func Sets(file string, pairs []interface{}) (err error) {
	db, err := Open(file, nil)
	if err != nil {
		return err
	}
	for i := range pairs {
		if i%2 != 0 {
			// on odd - append val and store key
			if pairs[i] == nil || pairs[i-1] == nil {
				break
			}
			err = db.Set(pairs[i-1], pairs[i])
			if err != nil {
				break
			}
		}
	}
	return err
}

// Get return value by key with opening if needed
// Return error if any.
func Get(f string, key, value interface{}) error {
	db, err := Open(f, nil)
	if err != nil {
		return err
	}
	return db.Get(key, value)
}

// Gets return key/value pairs in random order
// result contains key and value
// Gets not return error if key not found
// If no keys found return empty result
func Gets(file string, keys []interface{}) (result [][]byte) {
	db, err := Open(file, nil)
	if err != nil {
		return nil
	}
	for _, key := range keys {
		var v []byte
		err := db.Get(key, &v)
		if err == nil {
			k, err := KeyToBinary(key)
			if err == nil {
				val, err := ValToBinary(v)
				if err == nil {
					result = append(result, k)
					result = append(result, val)
				}
			}
		}
	}
	return result
}

// Counter return int64 incremented on incr with lazy open
func Counter(f string, key interface{}, incr int) (int64, error) {
	db, err := Open(f, nil)
	if err != nil {
		return 0, err
	}
	return db.Counter(key, incr)
}

// Delete remove key
// Returns error if key not found
func Delete(f string, key interface{}) error {
	db, err := Open(f, nil)
	if err != nil {
		return err
	}
	return db.Delete(key)
}

// Keys return keys in ascending  or descending order (false - descending,true - ascending)
// if limit == 0 return all keys
// if offset > 0 - skip offset records
// If from not nil - return keys after from (from not included)
func Keys(f string, from interface{}, limit, offset int, asc bool) ([][]byte, error) {
	db, err := Open(f, nil)
	if err != nil {
		return nil, err
	}
	return db.Keys(from, limit, offset, asc)
}

// Has return true if key exists.
// Return error if any.
func Has(f string, key interface{}) (bool, error) {
	db, err := Open(f, nil)
	if err != nil {
		return false, err
	}
	return db.Has(key)
}

// Count returns the number of items in the Db.
func Count(f string) (int, error) {
	db, err := Open(f, nil)
	if err != nil {
		return -1, err
	}
	return db.Count()
}

// Close - sync & close files.
// Return error if any.
func Close(f string) error {
	db, err := Open(f, nil)
	if err != nil {
		return err
	}
	return db.Close()
}

// BackupAll - backup all opened Db
// if dir not set it will be backup
// delete old backup file before run
// ignore all errors
func BackupAll(dir string) (err error) {
	if dir == "" {
		dir = "backup"
	}
	dbs.Lock()
	stores := dbs.dbs
	dbs.Unlock()
	for _, db := range stores {
		backup := dir + "/" + db.name
		DeleteFile(backup)
		keys, err := db.Keys(nil, 0, 0, true)
		if err == nil {
			for _, k := range keys {
				var b []byte
				db.Get(k, &b)
				Set(backup, k, b)
			}
		}
		Close(backup)
	}

	return err
}
