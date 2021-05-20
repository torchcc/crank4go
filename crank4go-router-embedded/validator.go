package router

type Validator interface {
	IsValid(string) bool
}

type IpValidator struct {
	ipWhiteList map[string]struct{}
}

// if ip set is nil, isValid will return true by default.
// if ip set is not nil and the given ip is not in the set, it will return false
func (v *IpValidator) IsValid(ip string) bool {
	if v.ipWhiteList == nil {
		return true
	}
	if _, ok := v.ipWhiteList[ip]; ok {
		return true
	} else {
		return false
	}
}

func (v *IpValidator) UpdateIpWhiteList(newIps []string) {
	m := make(map[string]struct{})
	for _, ip := range newIps {
		m[ip] = struct{}{}
	}
	v.ipWhiteList = m
}
