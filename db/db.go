package db

import (
	"GADS/util"

	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

var DBSession *r.Session

// Create a new connection to RethinkDB
func NewConnection() {
	var err error = nil
	DBSession, err = r.Connect(r.ConnectOpts{
		Address:  util.ConfigData.RethinkDB,
		Database: "gads",
	})

	if err != nil {
		panic("Could not connect to RethinkDB on " + util.ConfigData.RethinkDB + ", make sure it is set up and running, err: " + err.Error())
	}
}
