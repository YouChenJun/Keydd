package main

import (
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"

	"Keydd/go-mitmproxy/cert"
	log "github.com/sirupsen/logrus"
)

// 生成假的/用于测试的服务器证书

type Config struct {
	commonName string
}

func loadConfig() *Config {
	config := new(Config)
	flag.StringVar(&config.commonName, "commonName", "", "server commonName")
	flag.Parse()
	return config
}

func main() {
	log.SetLevel(log.InfoLevel)
	log.SetReportCaller(false)
	//log.SetOutput(os.Stdout)
	//暂时不需要任何日志输出
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}
	log.SetOutput(null)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	config := loadConfig()
	if config.commonName == "" {
		log.Fatal("commonName required")
	}

	caApi, err := cert.NewSelfSignCA("")
	if err != nil {
		panic(err)
	}
	ca := caApi.(*cert.SelfSignCA)

	cert, err := ca.DummyCert(config.commonName)
	if err != nil {
		panic(err)
	}

	os.Stdout.WriteString(fmt.Sprintf("%v-cert.pem\n", config.commonName))
	err = pem.Encode(os.Stdout, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	if err != nil {
		panic(err)
	}
	os.Stdout.WriteString(fmt.Sprintf("\n%v-key.pem\n", config.commonName))

	keyBytes, err := x509.MarshalPKCS8PrivateKey(&ca.PrivateKey)
	if err != nil {
		panic(err)
	}
	err = pem.Encode(os.Stdout, &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	if err != nil {
		panic(err)
	}
}
