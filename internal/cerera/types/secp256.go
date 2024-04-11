//go:build !gofuzz && cgo
// +build !gofuzz,cgo

package types

// Copyright 2015 Jeffrey Wilcke, Felix Lange, Gustav Simonsson. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

/*
#cgo CFLAGS: -I./libsecp256k1
#cgo CFLAGS: -I./libsecp256k1/src/

#ifdef __SIZEOF_INT128__
#  define HAVE___INT128
#  define USE_FIELD_5X52
#  define USE_SCALAR_4X64
#else
#  define USE_FIELD_10X26
#  define USE_SCALAR_8X32
#endif

#define USE_ENDOMORPHISM
#define USE_NUM_NONE
#define USE_FIELD_INV_BUILTIN
#define USE_SCALAR_INV_BUILTIN
#define NDEBUG
#include "./libsecp256k1/src/secp256k1.c"
#include "./libsecp256k1/src/modules/recovery/main_impl.h"
#include "ext.h"

typedef void (*callbackFunc) (const char* msg, void* data);
extern void secp256k1GoPanicIllegal(const char* msg, void* data);
extern void secp256k1GoPanicError(const char* msg, void* data);
*/

import "C"

// var context *C.secp256k1_context

// func init() {
// 	// around 20 ms on a modern CPU.
// 	context = C.secp256k1_context_create_sign_verify()
// 	C.secp256k1_context_set_illegal_callback(context, C.callbackFunc(C.secp256k1GoPanicIllegal), nil)
// 	C.secp256k1_context_set_error_callback(context, C.callbackFunc(C.secp256k1GoPanicError), nil)
// }

// func RecoverPubkey(msg []byte, sig []byte) ([]byte, error) {
// 	if len(msg) != 32 {
// 		return nil, ErrInvalidMsgLen
// 	}
// 	if err := checkSignature(sig); err != nil {
// 		return nil, err
// 	}

// 	var (
// 		pubkey  = make([]byte, 65)
// 		sigdata = (*C.uchar)(unsafe.Pointer(&sig[0]))
// 		msgdata = (*C.uchar)(unsafe.Pointer(&msg[0]))
// 	)
// 	if C.secp256k1_ext_ecdsa_recover(context, (*C.uchar)(unsafe.Pointer(&pubkey[0])), sigdata, msgdata) == 0 {
// 		return nil, ErrRecoverFailed
// 	}
// 	return pubkey, nil
// }
