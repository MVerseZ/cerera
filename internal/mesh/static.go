package mesh

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/cerera/config"
)

var meshLogger = log.New(os.Stdout, "[mesh] ", log.LstdFlags|log.Lmicroseconds)

const bIP = "192.168.1.6"
const bPort = "31100"

func Start(cfg *config.Config, ctx context.Context, port string) (*DHT, error) {
	currentIP := ""
	interfaces, err := net.InterfaceAddrs()
	if err != nil {
		meshLogger.Println("error getting interfaces", err)
		return nil, err
	}
	for _, iface := range interfaces {
		ip, _, err := net.ParseCIDR(iface.String())
		if err != nil {
			continue
		}
		if ip.To4() != nil {
			meshLogger.Println("ipv4", ip.String())
			currentIP = ip.String()
			break
		}
	}

	var bootstrapNodes []*NetworkNode
	if currentIP+":"+port != bIP+":"+bPort {
		bootstrapNode := NewNetworkNode(bIP, bPort)
		meshLogger.Println("Add node to bootstrap nodes: ", bootstrapNode.IP.String(), bootstrapNode.Port)
		bootstrapNodes = append(bootstrapNodes, bootstrapNode)
	}

	dht, err := NewDHT(&MemoryStore{}, &Options{
		BootstrapNodes: bootstrapNodes,
		ID:             cfg.NetCfg.ADDR.Bytes(),
		IP:             currentIP,
		Port:           port,
		UseStun:        false,
	})
	if err != nil {
		meshLogger.Println("error creating DHT", err)
		return nil, err
	}

	meshLogger.Println("Opening socket...")
	dht.CreateSocket()
	meshLogger.Println("..done")

	go func() {
		meshLogger.Println("Now listening on " + dht.GetNetworkAddr())
		err := dht.Listen()
		panic(err)
	}()

	if len(bootstrapNodes) > 0 {
		meshLogger.Println("Bootstrapping..")
		dht.Bootstrap()
		meshLogger.Println("..done")
	}

	return dht, nil
}
