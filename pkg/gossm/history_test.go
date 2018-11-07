package gossm

import (
	"database/sql"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestTableCreation(t *testing.T) {
	path := "foo.db"
	defer os.Remove(path)
	os.Remove(path)

	h, err := NewHistory(path)
	assert.NoError(t, err)

	err = h.Close()
	assert.NoError(t, err)

	db, err := sql.Open("sqlite3", path)
	assert.NoError(t, err)

	_, err = db.Query("select * from commands")
	assert.NoError(t, err)

	_, err = db.Query("select * from Invocations")
	assert.NoError(t, err)
}
