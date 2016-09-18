package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// TaobaoIPURL is the API endpoint to get external IP address from Taobao.
const TaobaoIPURL = "http://ip.taobao.com/service/getIpInfo.php?ip=myip"

func init() {
	TaobaoRequest, _ = http.NewRequest(http.MethodGet, TaobaoIPURL, nil)
}

type MyIP struct {
	sync.RWMutex
	ip  net.IP
	api Interface
}

// Interface defines the interface to get external IP address of running code.
type Interface interface {
	IP() (net.IP, error)
}

// Taobao implements Interface using Taobao's API
type Taobao struct {
	Interface

	Client *http.Client
}

func (m *MyIP) refreshFromTaobaoIP() error {
	ip, err := m.api.IP()
	if err != nil {
		return err
	}
	m.SetIP(ip)
	return nil
}

func (m *MyIP) GetIP() net.IP {
	m.RLock()
	defer m.RUnlock()
	return m.ip
}

func (m *MyIP) SetIP(ip net.IP) {
	m.Lock()
	m.ip = ip
	m.Unlock()
}

// New return a new MyIP instance to get external IP address from given API.
func New(api Interface) *MyIP {
	return &MyIP{api: api}
}

func (m *MyIP) StartTaobaoIPLoop(cb func(oldIP, newIP net.IP)) {
	go func() {
		oldIP := m.GetIP()
		for {
			if err := m.refreshFromTaobaoIP(); err != nil {
				log.Printf("refresh myip failed: %v", err)
			} else {
				newIP := m.GetIP()
				if !oldIP.Equal(newIP) {
					log.Printf("myip changed from %s to %s", oldIP, newIP)
					if cb != nil {
						go cb(oldIP, newIP)
					}
					oldIP = newIP
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()
}

// IP return external IPv4 address from Taobao's API
func (t Taobao) IP() (net.IP, error) {
	c := t.Client
	if c == nil {
		c = http.DefaultClient
	}

	resp, err := c.Do(TaobaoRequest)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %v", resp.Status)
	}

	var parsed struct {
		Code int `json:"code"`
		Data struct {
			IP string `json:"ip"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed in decoding Taobao response: %v", err)
	}
	ip := net.ParseIP(parsed.Data.IP)
	if ip == nil {
		return nil, fmt.Errorf("failed to parse %q as ip", parsed.Data.IP)
	}
	return ip, nil
}
