package main

import (
	"fmt"
	"log"

	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

var session *r.Session

var currentDevicesInfo []Device

func New(address string) {
	var err error = nil
	session, err = r.Connect(r.ConnectOpts{
		Address:  address,
		Database: "gads",
	})

	if err != nil {
		panic("Could not connect to db on " + address + ", err: " + err.Error())
	}
}

func GetDBDevicesOnStart() {
	cursor, err := r.Table("devices").Run(session)
	if err != nil {
		panic(err)
	}

	defer cursor.Close()

	err = cursor.All(&currentDevicesInfo)
	if err != nil {
		panic(err)
	}
}

func ReadDevices() {
	cursor, err := r.Table("devices").Run(session)
	if err != nil {
		panic(err)
	}

	defer cursor.Close()

	var devices []Device
	err = cursor.All(&devices)
	if err != nil {
		panic(err)
	}

	test := ConvertToJSONString(&devices)

	fmt.Println(test)
}

func ReadChanges() {
	res, err := r.Table("devices").Field("Connected").Changes().Run(session)
	if err != nil {
		panic(err)
	}

	var value interface{}

	if err != nil {
		log.Fatalln(err)
	}

	for res.Next(&value) {
		fmt.Println(value)
	}
}
