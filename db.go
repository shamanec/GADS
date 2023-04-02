package main

import (
	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

var session *r.Session

var currentDevicesInfo []Device

func InitDB() {
	var err error = nil
	session, err = r.Connect(r.ConnectOpts{
		Address:  ConfigData.RethinkDB,
		Database: "gads",
	})

	if err != nil {
		panic("Could not connect to db on " + ConfigData.RethinkDB + ", err: " + err.Error())
	}
}
