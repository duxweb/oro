package oro

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
)

type extensionTestDriver struct {
	db    *sql.DB
	owned bool
}

func (driver extensionTestDriver) Name() string { return "extension-test" }
func (driver extensionTestDriver) Open(ctx context.Context) (*sql.DB, error) {
	return driver.db, nil
}
func (driver extensionTestDriver) Dialect() Dialect { return fakeDialect{} }
func (driver extensionTestDriver) Inspector(db *sql.DB) Inspector {
	return nil
}
func (driver extensionTestDriver) TranslateError(err error) error { return err }
func (driver extensionTestDriver) Owned() bool                    { return driver.owned }

type closeTrackingSQLDriver struct {
	state *closeTrackingState
}

type closeTrackingState struct {
	closed bool
}

type closeTrackingConn struct {
	state *closeTrackingState
}

func (driver closeTrackingSQLDriver) Open(name string) (driver.Conn, error) {
	return &closeTrackingConn{state: driver.state}, nil
}

func (conn *closeTrackingConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("prepare unsupported")
}

func (conn *closeTrackingConn) Close() error {
	conn.state.closed = true
	return nil
}

func (conn *closeTrackingConn) Begin() (driver.Tx, error) {
	return nil, errors.New("tx unsupported")
}

func TestOpenInstallsExtensions(t *testing.T) {
	var installed bool
	db, err := Open(Config{
		Connections: map[string]ConnectionConfig{
			"default": {Driver: extensionTestDriver{}},
		},
		Extensions: []Extension{
			ExtensionFunc{
				ExtensionName: "test",
				Fn: func(db *DB) error {
					installed = db != nil
					return nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close(context.Background())
	if !installed {
		t.Fatal("expected extension to be installed")
	}
}

func TestOpenRejectsDuplicateExtensions(t *testing.T) {
	_, err := Open(Config{
		Connections: map[string]ConnectionConfig{
			"default": {Driver: extensionTestDriver{}},
		},
		Extensions: []Extension{
			ExtensionFunc{ExtensionName: "test"},
			ExtensionFunc{ExtensionName: "test"},
		},
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestOpenClosesOwnedConnectionWhenExtensionFails(t *testing.T) {
	state := &closeTrackingState{}
	sql.Register("oro_extension_close", closeTrackingSQLDriver{state: state})
	sqlDB, err := sql.Open("oro_extension_close", "")
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	_, err = Open(Config{
		Connections: map[string]ConnectionConfig{
			"default": {Driver: extensionTestDriver{db: sqlDB, owned: true}},
		},
		Pool: PoolConfig{PingOnOpen: true},
		Extensions: []Extension{
			ExtensionFunc{
				ExtensionName: "broken",
				Fn: func(db *DB) error {
					return errors.New("install failed")
				},
			},
		},
	})
	if !errors.Is(err, ErrHook) {
		t.Fatalf("expected hook error, got %v", err)
	}
	if err := sqlDB.Ping(); err == nil {
		t.Fatal("expected database to be closed")
	}
	if !state.closed {
		t.Fatal("expected underlying connection to be closed")
	}
}
