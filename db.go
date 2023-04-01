package main

import (
	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

var session *r.Session

var currentDevicesInfo []Device

func InitDB(address string) {
	var err error = nil
	session, err = r.Connect(r.ConnectOpts{
		Address:  address,
		Database: "gads",
	})

	if err != nil {
		panic("Could not connect to db on " + address + ", err: " + err.Error())
	}
}
