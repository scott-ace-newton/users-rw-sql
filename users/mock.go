package users

import (
	p "github.com/scott-ace-newton/users-rw-sql/persistence"
)

type mockSqlClient struct {
	expectedStatus p.Status
	expectedRecords []p.UserRecord
}

func(mc *mockSqlClient) CreateRecord(p.UserRecord) p.Status {
	return mc.expectedStatus
}

func(mc *mockSqlClient) UpdateRecord(string, map[string]string) p.Status {
	return mc.expectedStatus
}

func(mc *mockSqlClient) RetrieveRecords(map[string]string) ([]p.UserRecord, p.Status) {
	return mc.expectedRecords, mc.expectedStatus
}

func(mc *mockSqlClient) DeleteRecord(string) p.Status {
	return mc.expectedStatus
}

func(mc *mockSqlClient) ActiveConnection() bool {
	return true
}