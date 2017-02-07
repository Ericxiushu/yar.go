package distribunted_client

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/weixinhost/yar.go/host_sync"
	"github.com/weixinhost/yar.go/monitor"
)

const (
	defaultSyncInterval = 2 //默认从redis同步的时间
)

type hostAnalytics struct {
	lastUseTime time.Time //最后一次使用时间
	failCount   int       //连续失败总数
}

type Peer struct {
	pool          string
	name          string
	hostList      []string
	lastSyncTime  time.Time
	failAnalytics map[string]*hostAnalytics
	failMutex     sync.Mutex
	hostMutext    sync.Mutex
	hostLastIndex int
	lastAlarmTime time.Time
}

func NewPeer(pool, name string) *Peer {
	peer := new(Peer)
	peer.pool = pool
	peer.name = name
	peer.failAnalytics = make(map[string]*hostAnalytics)
	return peer
}

func (p *Peer) GetNextHost() (string, error) {

	now := time.Now()
	if int(now.Sub(p.lastSyncTime).Seconds()) > defaultSyncInterval {
		p.syncHostList()
	}

	p.hostMutext.Lock()
	defer p.hostMutext.Unlock()

	if len(p.hostList) < 1 {
		msg := fmt.Sprintf(" Pool:%s \n Yar Service: %s \n No Host Found", p.pool, p.name)
		p.Alerm("", msg)
		return "", errors.New("Host Not Found")
	}
	first := p.hostLastIndex
	for {
		next := p.hostLastIndex % len(p.hostList)
		p.hostLastIndex++
		ip := p.hostList[next]
		if p.isAllow(ip) {
			return ip, nil
		}
		if p.hostLastIndex == first {
			break
		}
	}
	msg := fmt.Sprintf(" Pool:%s \n Yar Service: %s \n No Health Host Found", p.pool, p.name)
	p.Alerm("", msg)
	return "", errors.New("No Health Host Found")
}

func (p *Peer) SyncHostList(list []string) {
	p.hostMutext.Lock()
	defer p.hostMutext.Unlock()
	p.hostList = list
}

func (p *Peer) Alerm(addr string, msg string) {

	monitor.RealTimeMonitor(p.pool, p.name, addr, msg)
}

func (p *Peer) syncHostList() {
	p.hostMutext.Lock()
	defer p.hostMutext.Unlock()
	lst, err := host_sync.GetHostList(p.pool, p.name)
	if err == nil {
		p.hostList = lst
		p.lastSyncTime = time.Now()
	}

	/*
		p.hostList = []string{
			"127.0.0.1:8501",
			"127.0.0.1:8502",
			"127.0.0.1:8503",
			"127.0.0.1:8504",
		}
	*/

}

func (p *Peer) SetFail(ip string) {
	p.failMutex.Lock()
	defer p.failMutex.Unlock()

	ff, ok := p.failAnalytics[ip]

	if !ok {
		p.failAnalytics[ip] = new(hostAnalytics)
		ff = p.failAnalytics[ip]
	}
	ff.lastUseTime = time.Now()
	ff.failCount++
}

func (p *Peer) Reset(ip string) {
	p.failMutex.Lock()
	defer p.failMutex.Unlock()
	ff, ok := p.failAnalytics[ip]
	if ok {
		ff.failCount = 0
	}
}

func (p *Peer) Len() int {
	return len(p.hostList)
}

/**

**/
func (p *Peer) isAllow(ip string) bool {

	p.failMutex.Lock()
	a := p.failAnalytics[ip]
	p.failMutex.Unlock()
	var c int
	var t time.Time

	if a != nil {
		c = a.failCount
		t = a.lastUseTime
	}

	n := time.Now()
	f := n.Sub(t).Seconds()
	if c < 1 {
		return true
	}

	if c <= 1 && f > 1.0 {
		return true
	}

	if c <= 2 && f > 2.0 {
		return true
	}

	//montior
	if c <= 5 && f > 10.0 {
		return true
	}

	//monitor
	if c <= 10 && f > 60.0 {
		return true
	}

	//monitor
	if c <= 20 && f > 180.0 {
		return true
	}

	if f > 360.0 {
		return true
	}

	if c > 10 {
		msg := fmt.Sprintf(" Pool:%s \n Yar Service Container: %s \n Failed Total: %d", p.pool, ip, c)
		p.Alerm(ip, msg)
	}

	return false

}