package persistence

import (
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"strings"
)

type Client struct {
	DB *sql.DB
}

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

type Clienter interface {
	CreateRecord(UserRecord) Status
	UpdateRecord(string, map[string]string) Status
	RetrieveRecords(map[string]string) ([]UserRecord, Status)
	DeleteRecord(string) Status
	ActiveConnection() bool
}

func NewClient(dsn string, credentials string) (Clienter, error) {
	connString := fmt.Sprintf("%s@tcp(%s)/factset?interpolateParams=true&parseTime=true", credentials, dsn)
	db, err := sql.Open("mysql", connString)
	if err != nil {
		log.WithError(err).Error("error connecting to db")
		return &Client{}, err
	}

	if err = db.Ping(); err != nil {
		log.WithError(err).Error("error establishing active connection to db")
		return &Client{}, err
	}

	//query1 := `CREATE IF NOT EXISTS SCHEMA 'test' ;`

	query := `CREATE TABLE Users (
    	user_id varchar(36)  NOT NULL,
    	first_name varchar(50) NOT NULL,
    	last_name varchar(50) NOT NULL,
    	email varchar(150) NOT NULL,
    	password varchar(50) NOT NULL,
    	nickname varchar(50) NOT NULL,
    	country varchar(50) NOT NULL,
  		PRIMARY KEY (user_id);`
	_, err = db.Exec(query)
	if err != nil {
		log.WithError(err).Error("error creating Users table")
		return &Client{}, err
	}
	return &Client{
		DB: db,
	}, nil
}

func (c *Client) CreateRecord(record UserRecord) Status {
	dbQuery := `INSERT INTO Users (user_id, first_name, last_name, email, password, nickname, country)
		VALUES (?, ?, ?, ?, ?, ?, ?);`
	_, err := c.DB.Exec(dbQuery, record.UserID, record.FirstName, record.LastName, record.EmailAddress, record.Password, record.NickName, record.Country)
	if err != nil {
		sqlError, _ := err.(*mysql.MySQLError)
		if sqlError.Number == 1062 {
			log.WithError(err).WithField("UserID", record.UserID).Errorf("user with this email: %s already exists!", record.EmailAddress)
			return ALREADY_EXISTS
		}
		log.WithError(err).WithField("UserID", record.UserID).Errorf("could not add user to DB")
		return BACKEND_ERROR
	}
	log.WithField("UserID", record.UserID).Infof("created record for user with email %s", record.EmailAddress)
	return CREATED
}

func (c *Client) UpdateRecord(userID string, fieldsToUpdate map[string]string) Status {
	var updates string
	for k,v := range fieldsToUpdate {
		updates = fmt.Sprintf("%s%s = '%s',", updates, k, v)
	}

	updateTemplate := fmt.Sprintf(`UPDATE Users
						SET %s
						WHERE user_id = ?;`, strings.TrimSuffix(updates, ","))
	log.WithField("UserID", userID).Debugf("update query: %s", updateTemplate)
	results, err := c.DB.Exec(updateTemplate, userID)
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

	statement, err := c.DB.Prepare(retrieveTemplate)
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

func (c *Client) DeleteRecord(userID string) Status {
	deleteTemplate := `DELETE FROM Users
					   WHERE user_id = ?;`
	results, err := c.DB.Exec(deleteTemplate, userID)
	if err != nil {
		log.WithError(err).WithField("UserID", userID).Error("could not delete user from DB")
		return BACKEND_ERROR
	}
	rows, err := results.RowsAffected()
	if err != nil {
		log.WithError(err).WithField("UserID", userID).Error("error processing request")
		return BACKEND_ERROR
	} else if rows == 0 {
		log.WithField("UserID", userID).Info("could not delete user from DB as they do not exist")
		return NOT_FOUND
	}
	log.WithField("UserID", userID).Info("user removed from DB")
	return DELETED
}

func (c *Client) ActiveConnection() bool {
	if err := c.DB.Ping(); err != nil {
		log.WithError(err).Error("Received error running RDS health check")
		return false
	}
	return true
}