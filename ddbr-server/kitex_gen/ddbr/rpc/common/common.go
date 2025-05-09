// Code generated by thriftgo (0.4.1). DO NOT EDIT.

package common

import (
	"fmt"
)

type Common struct {
	RespCode int32  `thrift:"resp_code,1" frugal:"1,default,i32" json:"resp_code"`
	Message  string `thrift:"message,2" frugal:"2,default,string" json:"message"`
}

func NewCommon() *Common {
	return &Common{}
}

func (p *Common) InitDefault() {
}

func (p *Common) GetRespCode() (v int32) {
	return p.RespCode
}

func (p *Common) GetMessage() (v string) {
	return p.Message
}
func (p *Common) SetRespCode(val int32) {
	p.RespCode = val
}
func (p *Common) SetMessage(val string) {
	p.Message = val
}

func (p *Common) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("Common(%+v)", *p)
}

var fieldIDToName_Common = map[int16]string{
	1: "resp_code",
	2: "message",
}

type Password struct {
	Password string `thrift:"password,1" frugal:"1,default,string" json:"password"`
}

func NewPassword() *Password {
	return &Password{}
}

func (p *Password) InitDefault() {
}

func (p *Password) GetPassword() (v string) {
	return p.Password
}
func (p *Password) SetPassword(val string) {
	p.Password = val
}

func (p *Password) String() string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprintf("Password(%+v)", *p)
}

var fieldIDToName_Password = map[int16]string{
	1: "password",
}
