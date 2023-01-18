package simpwebserv

import (
	"net/url"
	"strings"
)

func (request *Request) DecodeCookie() map[string]string {
	cookieMap := make(map[string]string)
	var cookieHeader string
	var ok bool
	if cookieHeader, ok = request.Header["Cookie"]; !ok {
		if cookieHeader, ok = request.Header["cookie"]; !ok {
			return cookieMap
		}
	}
	cookieList := strings.Split(cookieHeader, "; ")
	var cookieFirst string
	for i := 0; i < len(cookieList); i++ {
		singleCookieList := strings.Split(cookieList[i], "=")
		if len(singleCookieList) == 2 {
			cookieFirst, _ = url.QueryUnescape(singleCookieList[0])
			cookieMap[cookieFirst], _ = url.QueryUnescape(singleCookieList[1])
		}
	}
	return cookieMap
}
