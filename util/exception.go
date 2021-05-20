package util

type CrankerErr struct {
	Msg  string
	Code int
}

func (c *CrankerErr) Error() string {
	return c.Msg
}
func (c *CrankerErr) ErrCode() int {
	return c.Code
}

type CancelErr struct {
	Msg string
}

func (c CancelErr) Error() string {
	return "cancel error, detail: " + c.Msg
}

// An error is returned if caused by client policy (such as
// CheckRedirect), or failure to speak HTTP (such as a network
// connectivity problem). A non-2xx status code doesn't cause an
// error.
type HttpClientPolicyErr struct {
	Msg string
}

func (pe HttpClientPolicyErr) Error() string {
	return "err caused by client policy, (such as checkRedirect) or failure to speak http (such as a network connectivity problem), detail: " + pe.Msg
}

type NoRouteErr struct {
	Msg string
}

func (err NoRouteErr) Error() string {
	return err.Msg
}

type TimeoutErr struct {
	Msg string
}

func (err TimeoutErr) Error() string {
	return err.Msg
}
