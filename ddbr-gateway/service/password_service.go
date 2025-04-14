package service

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

var password string

func init() {
	passwordSlice := make([]int, 0)
	for i := 1; i <= 12; i++ {
		passwordSlice = append(passwordSlice, rand.Int()%10)
	}
	strSlice := make([]string, len(passwordSlice))

	// 将每个整数转换为字符串
	for i, num := range passwordSlice {
		strSlice[i] = strconv.Itoa(num)
	}
	password = strings.Join(strSlice, "")
	fmt.Println("password is :" + password)
}
func GetPassword() string {
	return password
}
func SetPassword(set string) {
	password = set
}
