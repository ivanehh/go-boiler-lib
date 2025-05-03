package db

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"reflect"
)

var ErrBadConfig = errors.New("the configuration provided is missing fields or has bad values in the provided fields")

type dbMode int

type QueryConstructor interface {
	Construct() string
}

type QueryWrapper interface {
	Wrap(*sql.Rows)
}

type QueryUnwrapper interface {
	Unwrap() any
}

type Query interface {
	QueryConstructor
	QueryWrapper
	QueryUnwrapper
}

const (
	stage dbMode = iota
	prod  dbMode = iota
)

// DatabaseConfig provides the necessary configuration for Database initiailization; All fields must be filled
type DatabaseConfig struct {
	Driver      string `json:"driver"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Credentials struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	} `json:"credentials"`
	/* 	 ConnectionStringTemplate example:"sqlserver://{{.Credentials.Name}}:{{.Credentials.Password}}@{{.Address}}/?database={{.Name}}" */
	ConnectionStringTemplate *template.Template
}

type Database struct {
	*sql.DB
	Config     DatabaseConfig
	connString string
	prepStmts  map[string]*sql.Stmt
	open       bool
}

func ValidateConfig(c DatabaseConfig) error {
	valid := len(c.Address) != 0 && len(c.Driver) != 0 && c.ConnectionStringTemplate != nil && len(c.Credentials.Name) != 0 && len(c.Credentials.Password) != 0
	if !valid {
		return ErrBadConfig
	}
	return nil
}

func NewDatabase(c DatabaseConfig, name string) (*Database, error) {
	if err := ValidateConfig(c); err != nil {
		return nil, err
	}
	connectionString := bytes.NewBuffer([]byte{})
	db := new(Database)
	db.Config = c
	err := db.Config.ConnectionStringTemplate.Execute(connectionString, db.Config)
	if err != nil {
		return nil, err
	}
	db.connString = connectionString.String()

	db.DB, err = sql.Open(db.Config.Driver, db.connString)
	if err != nil {
		return nil, err
	}
	db.open = true
	db.prepStmts = make(map[string]*sql.Stmt)
	return db, nil
}

func (pdb *Database) Close() error {
	err := pdb.DB.Close()
	if err != nil {
		return err
	}
	pdb.open = false
	return nil
}

func (pdb *Database) QueryWrappedValues(qc Query, params ...any) (QueryUnwrapper, error) {
	var err error
	// INFO: Commented out this mechanism as it created a bug where all queries have a name of empty string
	// if stmt, ok = pdb.prepStmts[reflect.TypeOf(qc).Name()]; !ok {
	// 	stmt, err = pdb.Prepare(qc.Construct())
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	pdb.prepStmts[reflect.TypeOf(qc).Name()] = stmt
	//
	// }
	q, err := pdb.Query(qc.Construct(), params...)
	if err != nil {
		return nil, err
	}
	qc.Wrap(q)
	return qc, nil
}

func (pdb *Database) ExecuteConstructor(qc QueryConstructor, params ...any) (sql.Result, error) {
	var stmt *sql.Stmt
	var ok bool
	var err error
	if err != nil {
		return nil, err
	}
	if stmt, ok = pdb.prepStmts[reflect.TypeOf(qc).Name()]; !ok {
		stmt, err = pdb.DB.Prepare(qc.Construct())
		if err != nil {
			return nil, fmt.Errorf("statement construction error:%w", err)
		}
		pdb.prepStmts[reflect.TypeOf(qc).Name()] = stmt
	}
	return stmt.Exec(params...)
}
