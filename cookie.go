package simpwebserv

import (
	"net/url"
	"strings"
)

func (request *Request) DecodeCookie() map[string]string {
	cookieMap := make(map[string]string)
	if cookie, ok := request.Header["Cookie"]; ok {
		cookieList := strings.Split(cookie, "; ")
		var cookieFirst string
		for i := 0; i < len(cookieList); i++ {
			singleCookieList := strings.Split(cookieList[i], "=")
			if len(singleCookieList) == 2 {
				cookieFirst, _ = url.QueryUnescape(singleCookieList[0])
				cookieMap[cookieFirst], _ = url.QueryUnescape(singleCookieList[1])
			}
		}
	}
	return cookieMap
}
