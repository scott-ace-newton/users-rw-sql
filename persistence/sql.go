package persistence

import (
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	//mysql driver
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"strings"
)

//Client for SQL database
type Client struct {
	db *sql.DB
}

//Status abstracts business logic layer from http status codes
//Status must be exported for handler tests
type Status int
const (
	CREATED Status = iota
	ALREADY_EXISTS
	BACKEND_ERROR
	NOT_FOUND
	UPDATED
	OK
	DELETED
)

//Clienter provides an interface of Client functions. Useful for mocking
type Clienter interface {
	CreateRecord(UserRecord) Status
	UpdateRecord(string, map[string]string) Status
	RetrieveRecords(map[string]string) ([]UserRecord, Status)
	DeleteRecord(string) Status
	ActiveConnection() bool
}

//NewClient returns a MySQL client
func NewClient(dsn string, credentials string) (Clienter, error) {
	connString := fmt.Sprintf("%s@/%s?interpolateParams=true&parseTime=true", credentials, dsn)
	fmt.Printf("connecting on %s", connString)
	db, err := sql.Open("mysql", connString)
	if err != nil {
		log.WithError(err).Error("error connecting to db")
		return &Client{}, err
	}

	if err = db.Ping(); err != nil {
		log.WithError(err).Error("error establishing active connection to db")
		return &Client{}, err
	}


	//query1 := `CREATE IF NOT EXISTS SCHEMA test;`

	query := `CREATE TABLE IF NOT EXISTS Users (
    	user_id varchar(36)  NOT NULL,
    	first_name varchar(50) NOT NULL,
    	last_name varchar(50) NOT NULL,
    	email varchar(150) NOT NULL,
    	password varchar(50) NOT NULL,
    	nickname varchar(50) NOT NULL,
    	country varchar(50) NOT NULL,
  		PRIMARY KEY (user_id))`
	_, err = db.Exec(query)
	if err != nil {
		log.WithError(err).Error("error creating Users table")
		return &Client{}, err
	}
	return &Client{
		db: db,
	}, nil
}

//CreateRecord will attempt to add the provided user to the DB
func (c *Client) CreateRecord(record UserRecord) Status {
	dbQuery := `INSERT INTO Users (user_id, first_name, last_name, email, password, nickname, country)
		VALUES (?, ?, ?, ?, ?, ?, ?);`
	_, err := c.db.Exec(dbQuery, record.UserID, record.FirstName, record.LastName, record.EmailAddress, record.Password, record.NickName, record.Country)
	if err != nil {
		sqlError, _ := err.(*mysql.MySQLError)
		if sqlError.Number == 1062 {
			log.WithError(err).WithField("UserID", record.UserID).Errorf("user with this email: %s already exists!", record.EmailAddress)
			return ALREADY_EXISTS
		}
		log.WithError(err).WithField("UserID", record.UserID).Errorf("could not add user to db")
		return BACKEND_ERROR
	}
	log.WithField("UserID", record.UserID).Infof("created record for user with email %s", record.EmailAddress)
	return CREATED
}

//UpdateRecord will attempt to edit certain fields of the provided user in the DB
func (c *Client) UpdateRecord(userID string, fieldsToUpdate map[string]string) Status {
	var updates string
	for k,v := range fieldsToUpdate {
		updates = fmt.Sprintf("%s%s = '%s',", updates, k, v)
	}

	updateTemplate := fmt.Sprintf(`UPDATE Users
						SET %s
						WHERE user_id = ?;`, strings.TrimSuffix(updates, ","))
	log.WithField("UserID", userID).Debugf("update query: %s", updateTemplate)
	results, err := c.db.Exec(updateTemplate, userID)
	if err != nil {
		log.WithError(err).WithField("UserID", userID).Errorf("could not update user due to error running query")
		return BACKEND_ERROR
	}
	rows, err := results.RowsAffected()
	if err != nil {
		log.WithError(err).WithField("UserID", userID).Errorf("could not update user due to error with result set")
		return BACKEND_ERROR
	} else if rows == 0 {
		log.WithField("UserID", userID).Infof("could not update user as they do not exist")
		return NOT_FOUND
	}
	log.WithField("UserID", userID).Infof("updated user as follows: %s", updates)
 	return UPDATED
}

//RetrieveRecords will find all users matching the provided parameters in the DB
func (c *Client) RetrieveRecords(fieldsToUpdate map[string]string) ([]UserRecord, Status) {
	var results []UserRecord
	var whereClause string
	clause := "WHERE "
	for k,v := range fieldsToUpdate {
		whereClause = fmt.Sprintf("%s %s='%s'", clause, k, v)
		clause = " AND"
	}

	retrieveTemplate := fmt.Sprintf(`SELECT user_id as userID, first_name AS firstName, last_name AS lastName, email, password, nickname, country
						  FROM Users
						  %s;`, whereClause)
	log.Debugf("retrieve query is %s", retrieveTemplate)

	statement, err := c.db.Prepare(retrieveTemplate)
	if err != nil {
		log.WithError(err).Error("failed to prepare query template")
		return results, BACKEND_ERROR
	}
	defer statement.Close()

	var userID, firstName, lastName, email, password, nickname, country sql.NullString

	rows, err := statement.Query()
	defer rows.Close()
	if err != nil {
		log.WithError(err).Error("failed to execute query template")
		return results, BACKEND_ERROR
	}

	for rows.Next() {
		rows.Scan(&userID, &firstName, &lastName, &email, &password, &nickname, &country)
		results = append(results, UserRecord{
			UserID: validateString(userID),
			FirstName: validateString(firstName),
			LastName: validateString(lastName),
			EmailAddress:  validateString(email),
			Password: validateString(password),
			NickName: validateString(nickname),
			Country: validateString(country),
		})
	}
	if len(results) == 0 {
		log.Infof("found no users matching criteria: %s", fieldsToUpdate)
		return results, NOT_FOUND
	}

	log.Infof("users matching criteria are: %v", results)
	return results, OK
}

func validateString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

//DeleteRecord will attempt to remove the provided user from the DB
func (c *Client) DeleteRecord(userID string) Status {
	deleteTemplate := `DELETE FROM Users
					   WHERE user_id = ?;`
	results, err := c.db.Exec(deleteTemplate, userID)
	if err != nil {
		log.WithError(err).WithField("UserID", userID).Error("could not delete user from db")
		return BACKEND_ERROR
	}
	rows, err := results.RowsAffected()
	if err != nil {
		log.WithError(err).WithField("UserID", userID).Error("error processing request")
		return BACKEND_ERROR
	} else if rows == 0 {
		log.WithField("UserID", userID).Info("could not delete user from db as they do not exist")
		return NOT_FOUND
	}
	log.WithField("UserID", userID).Info("user removed from db")
	return DELETED
}

//ActiveConnection will check if still connected to DB
func (c *Client) ActiveConnection() bool {
	if err := c.db.Ping(); err != nil {
		log.WithError(err).Error("could not connect to db")
		return false
	}
	return true
}