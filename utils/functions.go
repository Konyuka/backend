package utils

import (
	"fmt"
	"net"
	"strings"
)

// GetLocalIP - returns local IP address
func GetLocalIP() string {

	conn, _ := net.Dial("udp", "8.8.8.8:80")

	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return strings.Split(localAddr.String(), ":")[0]
}

// InSlice - checks whether param exists in array slice
func InSlice(param string, array []string) bool {

	for i := range array {

		if param == array[i] {
			return true
		}
	}
	return false
}

// SQLINify - formats string to be used in SQL IN
func SQLINify(p []string) string {
	r := "("
	for i := range p {
		r += fmt.Sprintf("%v,", p[i])
	}
	return r[:strings.LastIndex(r, ",")] + ")"
}

// distinctArray - removes duplicate in a slice
func distinctArray(s []string) []string {

	var (
		check = make(map[string]int)
		res   = make([]string, 0)
	)

	for _, val := range s {
		check[val] = 1
	}

	for val := range check {
		res = append(res, val)
	}

	return res
}

// Find - check if slice contains item
func Find(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
