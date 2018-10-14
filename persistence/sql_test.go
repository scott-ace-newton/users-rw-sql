package persistence

import (
	"database/sql"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

const (
	janeDoe = "16f701dc-5e71-497b-a197-ef7b8618cbea"
	caesar  = "ff7dfd22-9134-429b-9482-0888ffdfc64b"
)

var client Client
var noMatch []UserRecord

func init() {
	log.SetLevel(log.DebugLevel)
}

func TestClient_GetUsersFromDB(t *testing.T) {
	var err error
	client, err = NewTestClient()
	if err != nil {
		log.Fatal("could not start test db")
	}
	assert.NoError(t, client.populateUserTable(), "test failed: could not add records to db")
	defer client.clearTestDatabase()

	tests := []struct {
		testName       string
		parameters     map[string]string
		resultFilePath string
		expectedStatus Status
	}{
		{
			testName: "GetUsers_JaneDoe",
			parameters: map[string]string{
				"user_id": janeDoe,
			},
			resultFilePath: "./fixtures/janeDoe.json",
			expectedStatus: OK,
		},
		{
			testName: "GetUser_UkUsers",
			parameters: map[string]string{
				"country": "United Kingdom",
			},
			resultFilePath: "./fixtures/ukUsers.json",
			expectedStatus: OK,
		},
		{
			testName: "GetUser_NoMatch",
			parameters: map[string]string{
				"country": "France",
			},
			resultFilePath: "",
			expectedStatus: NOT_FOUND,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			record, status := client.RetrieveRecords(test.parameters)
			assert.Equal(t, test.expectedStatus, status, "test failed: could not retrieve users")
			if test.resultFilePath != "" {
				expectedRecord, err := readFileAndDecode(t, test.resultFilePath)
				assert.NoError(t, err, "test failed: could not decode user json")
				assert.Equal(t, expectedRecord, record, "test failed: found record does not match expected")
				return
			}
			assert.Equal(t, noMatch, record, "test failed: found record does not match expected")
		})
	}
}

func TestClient_AddUpdateDeleteUsers(t *testing.T) {
	var err error
	var status Status
	client, err = NewTestClient()
	if err != nil {
		log.Fatal("could not start test db")
	}
	defer client.clearTestDatabase()
	startingUser := UserRecord{
		UserID: "ff7dfd22-9134-429b-9482-0888ffdfc64b",
		FirstName: "Julius",
		LastName: "Caesar",
		EmailAddress: "caesar@gmail.com",
		Password: "password4",
		NickName: "KingOfRome",
		Country: "Italy",
	}
	//can create user
	status = client.CreateRecord(startingUser)
	assert.Equal(t, CREATED, status, "test failed: could not create user: "+caesar)

	//can return new user
	readRecord, status := client.RetrieveRecords(map[string]string{"user_id": caesar})
	assert.Equal(t, OK, status, "test failed: could not retrieve user: "+caesar)
	assert.Equal(t, startingUser, readRecord[0])

	//return error when re-creating existing user_id
	status = client.CreateRecord(startingUser)
	assert.Equal(t, ALREADY_EXISTS, status, "test failed: could not re-create user with same id")


	updatedUser := UserRecord{
		UserID: "ff7dfd22-9134-429b-9482-0888ffdfc64b",
		FirstName: "Julius",
		LastName: "Caesar",
		EmailAddress: "caesar@gmail.com",
		Password: "password4",
		NickName: "eTuBrute",
		Country: "Italy",
	}
	//can update field
	status = client.UpdateRecord(caesar, map[string]string{"nickname": "eTuBrute"})
	assert.Equal(t, UPDATED, status, "test failed: could not update user")

	//field has been updated
	updatedRecord, status := client.RetrieveRecords(map[string]string{"user_id": caesar})
	assert.Equal(t, OK, status, "test failed: could not update user: "+caesar)
	assert.Equal(t, updatedUser, updatedRecord[0])

	newUpdatedUser := UserRecord{
		UserID: "ff7dfd22-9134-429b-9482-0888ffdfc64b",
		FirstName: "Augustus",
		LastName: "Caesar",
		EmailAddress: "caesar@gmail.com",
		Password: "password4",
		NickName: "KingOfRome",
		Country: "Italy",
	}
	//can update multiple fields
	status = client.UpdateRecord(caesar, map[string]string{"nickname": "KingOfRome","first_name":"Augustus"})
	assert.Equal(t, UPDATED, status, "test failed: could not update user")

	//both fields have been updated
	newUpdatedRecord, status := client.RetrieveRecords(map[string]string{"user_id": caesar})
	assert.Equal(t, OK, status, "test failed: could not retrieve user: "+caesar)
	assert.Equal(t, newUpdatedUser, newUpdatedRecord[0])

	//can delete record from db
	status = client.DeleteRecord(caesar)
	assert.Equal(t, DELETED, status,"test failed: could not delete user")

	//no results were returned for deleted record
	deletedRecord, status := client.RetrieveRecords(map[string]string{"user_id": caesar})
	assert.Equal(t, NOT_FOUND, status, "test failed: should not retrieve user: "+caesar)
	assert.Equal(t, noMatch, deletedRecord)

	//Deleting non-existing user results in sql no rows error
	status = client.DeleteRecord(caesar)
	assert.Equal(t, NOT_FOUND, status)
}

func NewTestClient() (Client, error) {
	connString := "root@/test?interpolateParams=true&parseTime=true"
	c, err := sql.Open("mysql", connString)
	if err != nil {
		log.WithError(err).Errorf("error connecting to db: %s", connString)
		return Client{}, err
	}

	if err = c.Ping(); err != nil {
		log.WithError(err).Errorf("error establishing valid connection to db: %s", connString)
		return Client{}, err
	}

	query := `CREATE TABLE Users (
    	user_id varchar(36)  NOT NULL,
    	first_name varchar(50) NOT NULL,
    	last_name varchar(50) NOT NULL,
    	email varchar(150) NOT NULL,
    	password varchar(50) NOT NULL,
    	nickname varchar(50) NOT NULL,
    	country varchar(50) NOT NULL,
  		PRIMARY KEY (user_id))`
	_, err = c.Exec(query)
	if err != nil {
		log.WithError(err).Error("error creating user table")
		return Client{}, err
	}

	return Client{
		db: c,
	}, nil
}

func (c *Client) clearTestDatabase() {
	query := "DROP TABLE IF EXISTS Users"
	_, err := c.db.Exec(query)
	if err != nil {
		log.Fatalf("failed to clear up test data tables with error: %v", err)
	}
}

func (c *Client) populateUserTable() error {
	dbQuery := `INSERT INTO Users (user_id, first_name, last_name, email, password, nickname, country)
		VALUES ('e41e62c8-6cf2-4fd7-a88b-41b86fcaa34d','John','Smith','john.smith@gmail.com','password1','smithy12345','United Kingdom'),
		('16f701dc-5e71-497b-a197-ef7b8618cbea','Jane','Doe','jane.doe@gmail.com','password2','GIJane','United States of America'),
		('b16dc0b3-e0ab-4dbd-89e3-d031a28cbc59','James','Bond','j.bond@mi6.co.uk','password007','BondJamesBond','United Kingdom'),
		('325ef78c-f0ac-424b-814d-7c7cd03ec44d','Cleo','Patra','cleopatra@gmail.com','password3','Cle0','Egypt'),
		('ff7dfd22-9134-429b-9482-0888ffdfc64b','Julius','Caesar','caesar@gmail.com','password4','ETuBrute','Italy');`
	_, err := c.db.Exec(dbQuery)
	if err != nil {
		fmt.Println("Error 2")
	}
	return err
}

func readFileAndDecode(t *testing.T, pathToFile string) ([]UserRecord, error) {
	f, err := os.Open(pathToFile)
	assert.NoError(t, err, "test failed: could not open file " + pathToFile)
	defer f.Close()
	dec := json.NewDecoder(f)
	ur := []UserRecord{}
	err = dec.Decode(&ur)
	return ur, err
}
