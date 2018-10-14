package users

import (
	p "github.com/scott-ace-newton/users-rw-sql/persistence"
)

type mockSQLClient struct {
	expectedStatus p.Status
	expectedRecords []p.UserRecord
}

func(mc *mockSQLClient) CreateRecord(p.UserRecord) p.Status {
	return mc.expectedStatus
}

func(mc *mockSQLClient) UpdateRecord(string, map[string]string) p.Status {
	return mc.expectedStatus
}

func(mc *mockSQLClient) RetrieveRecords(map[string]string) ([]p.UserRecord, p.Status) {
	return mc.expectedRecords, mc.expectedStatus
}

func(mc *mockSQLClient) DeleteRecord(string) p.Status {
	return mc.expectedStatus
}

func(mc *mockSQLClient) ActiveConnection() bool {
	return true
}