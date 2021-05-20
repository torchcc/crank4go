package darklaunch_manager

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

type IpListener interface {
	// trigger something after an ip is marked as dark ip
	AfterDarkIpAdded(addedIp string)

	// trigger something after a ip is revoked from dark ip list
	AfterDarkIpRevoked(revokedIp string)
}

type ServiceListener interface {

	// trigger something after an ip is marked as dark ip
	AfterDarkServiceAdded(addedService string)

	// trigger something after a ip is revoked from dark ip list
	AfterDarkServiceRevoked(revokedService string)
}

type DarkLaunchManager struct {
	currentIps      map[string]struct{}
	currentServices map[string]struct{}
	ipListener      IpListener
	serviceListener ServiceListener
	path            string
}

func NewDarkLaunchManager() *DarkLaunchManager {
	return &DarkLaunchManager{
		currentIps:      make(map[string]struct{}),
		currentServices: make(map[string]struct{}),
	}
}

func NewDarkLaunchManager2(path string) *DarkLaunchManager {
	return &DarkLaunchManager{
		path:            path,
		currentIps:      make(map[string]struct{}),
		currentServices: make(map[string]struct{}),
	}
}

func (m *DarkLaunchManager) IpList() []string {
	list := make([]string, 0, 8)
	for ip := range m.currentIps {
		list = append(list, ip)
	}
	return list
}

func (m *DarkLaunchManager) ServiceList() []string {
	list := make([]string, 0, 8)
	for service := range m.currentServices {
		list = append(list, service)
	}
	return list
}

func (m *DarkLaunchManager) AddIp(ip string) error {
	if isValidIp(ip) {
		m.currentIps[ip] = struct{}{}
		m.ipListener.AfterDarkIpAdded(ip)
		return nil
	} else {
		return errors.New("invalid ip address: " + ip)
	}
}

func (m *DarkLaunchManager) AddService(service string) error {
	if isValidService(service) {
		m.currentServices[service] = struct{}{}
		m.serviceListener.AfterDarkServiceAdded(service)
		return nil
	} else {
		return errors.New("invalid service: " + service)
	}
}

func (m *DarkLaunchManager) RemoveIp(ip string) error {
	if _, ok := m.currentIps[ip]; ok {
		delete(m.currentIps, ip)
		m.ipListener.AfterDarkIpRevoked(ip)
		return nil
	} else {
		return errors.New("ip: " + ip + " is not in current list")
	}
}

func (m *DarkLaunchManager) RemoveService(service string) error {
	if _, ok := m.currentServices[service]; ok {
		delete(m.currentServices, service)
		m.serviceListener.AfterDarkServiceRevoked(service)
		return nil
	} else {
		return errors.New("service: " + service + " is not in current list")
	}
}

func (m *DarkLaunchManager) IsDarkModeOn() bool {
	return len(m.currentServices) != 0 || len(m.currentIps) != 0
}

// judge if ip is in dark mode
func (m *DarkLaunchManager) ContainsIp(ip string) bool {
	if _, ok := m.currentIps[ip]; ok {
		return true
	}
	return false
}

// judge if ip is in dark mode
func (m *DarkLaunchManager) ContainsService(service string) bool {
	if _, ok := m.currentServices[service]; ok {
		return true
	}
	return false
}

func (m *DarkLaunchManager) SetServiceListener(serviceListener ServiceListener) *DarkLaunchManager {
	m.serviceListener = serviceListener
	return m
}

func (m *DarkLaunchManager) SetIpListener(ipListener IpListener) *DarkLaunchManager {
	m.ipListener = ipListener
	return m
}

func isValidService(service string) bool {
	if matched, err := regexp.MatchString("^[a-zA-Z]+((-|_)?\\w*)*$", service); err != nil || !matched {
		return false
	}
	return true
}

func isValidIp(ip string) bool {
	if ip == "" {
		return false
	}
	if groups := strings.Split(ip, "."); len(groups) != 4 {
		return false
	} else {
		for _, s := range groups {
			if len(s) == 0 {
				return false
			}
			if i, err := strconv.Atoi(s); err != nil || i < 0 || i > 255 {
				return false
			}
		}
		return true
	}
}
