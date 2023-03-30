package main

import (
	"fmt"
	"log"

	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

var session *r.Session

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
	res, err := r.Table("devices").Changes().Run(session)
	if err != nil {
		panic(err)
	}

	//var value interface{}
	var value *Device

	if err != nil {
		log.Fatalln(err)
	}

	for res.Next(&value) {
		fmt.Println(value)
	}
}

func ReadChanges2() {
	res, err := r.Table("devices").Field("State").Changes().Run(session)
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
