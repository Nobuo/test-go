package mysql

import (
	"bytes"
	"context"
	"crypto/sha1"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

func init() {
	sql.Register("mysql", &Driver{})
}

// Driver implements database/sql/driver.Driver using the MySQL client/server protocol.
// This is a minimal implementation tailored for the skeleton project. It currently
// supports authentication via the mysql_native_password plugin and basic connection
// health checks (Ping). Query execution and prepared statements are not implemented.
// For production use you should replace this driver with the upstream
// github.com/go-sql-driver/mysql module.
type Driver struct{}

// Open establishes a new connection to the MySQL server using a limited DSN syntax:
//
//	user:password@tcp(host:port)/database
func (d *Driver) Open(dsn string) (driver.Conn, error) {
	cfg, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}

	conn, err := dial(cfg)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// conn is a live connection to the server.
type conn struct {
	netConn net.Conn
	charset byte
}

// Ensure conn implements the required interfaces.
var (
	_ driver.Conn           = (*conn)(nil)
	_ driver.Pinger         = (*conn)(nil)
	_ driver.ExecerContext  = (*conn)(nil)
	_ driver.QueryerContext = (*conn)(nil)
)

func (c *conn) Close() error {
	if c.netConn == nil {
		return nil
	}
	// Best-effort COM_QUIT; ignore errors because the connection is closing.
	_ = c.simpleCommand(comQuit, nil, 1*time.Second)
	err := c.netConn.Close()
	c.netConn = nil
	return err
}

func (c *conn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not implemented in skeleton mysql driver")
}

func (c *conn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions not implemented in skeleton mysql driver")
}

func (c *conn) Ping(ctx context.Context) error {
	if c.netConn == nil {
		return errors.New("connection is closed")
	}
	return c.simpleCommandWithContext(ctx, comPing, nil)
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return nil, errors.New("ExecContext not implemented in skeleton mysql driver")
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return nil, errors.New("QueryContext not implemented in skeleton mysql driver")
}

func (c *conn) simpleCommandWithContext(ctx context.Context, cmd byte, payload []byte) error {
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		if err := c.netConn.SetDeadline(deadline); err != nil {
			return err
		}
		defer c.netConn.SetDeadline(time.Time{})
	}

	return c.simpleCommand(cmd, payload, 0)
}

func (c *conn) simpleCommand(cmd byte, payload []byte, timeout time.Duration) error {
	if timeout > 0 {
		if err := c.netConn.SetDeadline(time.Now().Add(timeout)); err != nil {
			return err
		}
		defer c.netConn.SetDeadline(time.Time{})
	}

	pkt := append([]byte{cmd}, payload...)
	if err := writePacket(c.netConn, 0, pkt); err != nil {
		return err
	}

	resp, _, err := readPacket(c.netConn)
	if err != nil {
		return err
	}

	if len(resp) == 0 {
		return errors.New("empty response from mysql server")
	}

	switch resp[0] {
	case okHeader:
		return nil
	case errHeader:
		return parseError(resp)
	default:
		return fmt.Errorf("unexpected mysql response header: 0x%02x", resp[0])
	}
}

// Configuration used for opening a connection.
type config struct {
	user string
	pass string
	host string
	port string
	db   string
}

func parseDSN(dsn string) (*config, error) {
	at := strings.Index(dsn, "@")
	if at < 0 {
		return nil, fmt.Errorf("invalid DSN: missing '@': %q", dsn)
	}

	userInfo := dsn[:at]
	addrPart := dsn[at+1:]

	if !strings.HasPrefix(addrPart, "tcp(") {
		return nil, fmt.Errorf("invalid DSN network, expected tcp(): %q", dsn)
	}

	closing := strings.Index(addrPart, ")")
	if closing < 0 {
		return nil, fmt.Errorf("invalid DSN: missing ')' in network address: %q", dsn)
	}

	addr := addrPart[len("tcp("):closing]
	rest := addrPart[closing+1:]

	if !strings.HasPrefix(rest, "/") {
		return nil, fmt.Errorf("invalid DSN: missing database name: %q", dsn)
	}

	dbAndParams := rest[1:]
	dbName := dbAndParams
	if q := strings.Index(dbAndParams, "?"); q >= 0 {
		dbName = dbAndParams[:q]
	}

	user := userInfo
	pass := ""
	if colon := strings.Index(userInfo, ":"); colon >= 0 {
		user = userInfo[:colon]
		pass = userInfo[colon+1:]
	}

	host := addr
	port := "3306"
	if strings.Contains(addr, ":") {
		h, p, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		host, port = h, p
	}

	if host == "" {
		host = "127.0.0.1"
	}

	return &config{user: user, pass: pass, host: host, port: port, db: dbName}, nil
}

func dial(cfg *config) (*conn, error) {
	d := &net.Dialer{Timeout: 5 * time.Second}
	netConn, err := d.Dial("tcp", net.JoinHostPort(cfg.host, cfg.port))
	if err != nil {
		return nil, err
	}

	if err := netConn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		netConn.Close()
		return nil, err
	}

	greeting, seq, err := readPacket(netConn)
	if err != nil {
		netConn.Close()
		return nil, err
	}
	if seq != 0 {
		netConn.Close()
		return nil, fmt.Errorf("unexpected handshake packet sequence %d", seq)
	}

	hs, err := parseHandshake(greeting)
	if err != nil {
		netConn.Close()
		return nil, err
	}

	if hs.plugin != "mysql_native_password" {
		netConn.Close()
		return nil, fmt.Errorf("unsupported auth plugin %q; configure the server to use mysql_native_password", hs.plugin)
	}

	response, err := buildHandshakeResponse(cfg, hs)
	if err != nil {
		netConn.Close()
		return nil, err
	}

	if err := writePacket(netConn, seq+1, response); err != nil {
		netConn.Close()
		return nil, err
	}

	authResp, _, err := readPacket(netConn)
	if err != nil {
		netConn.Close()
		return nil, err
	}

	if len(authResp) == 0 {
		netConn.Close()
		return nil, errors.New("empty authentication response from mysql server")
	}

	switch authResp[0] {
	case okHeader:
		// success
	case errHeader:
		err = parseError(authResp)
		netConn.Close()
		return nil, err
	case moreDataHeader:
		netConn.Close()
		return nil, errors.New("additional authentication steps required; not supported by skeleton driver")
	default:
		netConn.Close()
		return nil, fmt.Errorf("unexpected authentication response header 0x%02x", authResp[0])
	}

	if err := netConn.SetDeadline(time.Time{}); err != nil {
		netConn.Close()
		return nil, err
	}

	return &conn{netConn: netConn, charset: hs.charset}, nil
}

type handshake struct {
	capability uint32
	charset    byte
	salt       []byte
	plugin     string
}

func parseHandshake(data []byte) (*handshake, error) {
	var pos int
	if len(data) < 1 {
		return nil, errors.New("malformed handshake packet")
	}
	pos++ // skip protocol version

	// server version (null-terminated)
	nul := bytes.IndexByte(data[pos:], 0x00)
	if nul < 0 {
		return nil, errors.New("handshake missing server version terminator")
	}
	pos += nul + 1

	if len(data[pos:]) < 4 {
		return nil, errors.New("handshake missing connection id")
	}
	pos += 4 // connection id

	if len(data[pos:]) < 8 {
		return nil, errors.New("handshake missing auth plugin data")
	}
	salt1 := data[pos : pos+8]
	pos += 8

	pos++ // filler

	if len(data[pos:]) < 2 {
		return nil, errors.New("handshake missing capability flags")
	}
	capLow := binary.LittleEndian.Uint16(data[pos:])
	pos += 2

	var charset byte
	var capability uint32
	capability = uint32(capLow)

	if len(data[pos:]) > 0 {
		if len(data[pos:]) < 1+2+2 {
			return nil, errors.New("handshake v10 truncated")
		}
		charset = data[pos]
		pos++
		pos += 2 // status flags
		capHigh := binary.LittleEndian.Uint16(data[pos:])
		pos += 2
		capability |= uint32(capHigh) << 16

		var authPluginDataLen byte
		if capability&clientPluginAuth != 0 {
			authPluginDataLen = data[pos]
		}
		pos++
		if len(data[pos:]) < 10 {
			return nil, errors.New("handshake missing reserved bytes")
		}
		pos += 10

		salt2Len := int(authPluginDataLen) - 8
		if salt2Len < 13 {
			salt2Len = 13
		}
		if salt2Len > len(data[pos:]) {
			salt2Len = len(data[pos:])
		}
		salt2 := data[pos : pos+salt2Len]
		pos += salt2Len

		plugin := ""
		if capability&clientPluginAuth != 0 {
			if idx := bytes.IndexByte(data[pos:], 0x00); idx >= 0 {
				plugin = string(data[pos : pos+idx])
				pos += idx + 1
			} else {
				plugin = string(data[pos:])
				pos = len(data)
			}
		}

		salt := append([]byte{}, salt1...)
		salt = append(salt, salt2...)

		return &handshake{
			capability: capability,
			charset:    charset,
			salt:       salt,
			plugin:     plugin,
		}, nil
	}

	salt := append([]byte{}, salt1...)
	return &handshake{
		capability: capability,
		charset:    charset,
		salt:       salt,
		plugin:     "mysql_native_password",
	}, nil
}

func buildHandshakeResponse(cfg *config, hs *handshake) ([]byte, error) {
	capability := clientLongPassword | clientLongFlag | clientProtocol41 | clientSecureConnection | clientDeprecateEOF | clientPluginAuth
	if cfg.db != "" {
		capability |= clientConnectWithDB
	}

	authResp, err := scrambleNativePassword(cfg.pass, hs.salt)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	tmp := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmp, uint32(capability))
	buf.Write(tmp)

	buf.Write([]byte{0x00, 0x00, 0x00, 0x00}) // max packet size
	buf.WriteByte(hs.charset)
	buf.Write(make([]byte, 23))

	buf.WriteString(cfg.user)
	buf.WriteByte(0x00)

	buf.WriteByte(byte(len(authResp)))
	buf.Write(authResp)

	if capability&clientConnectWithDB != 0 {
		buf.WriteString(cfg.db)
		buf.WriteByte(0x00)
	}

	buf.WriteString("mysql_native_password")
	buf.WriteByte(0x00)

	return buf.Bytes(), nil
}

func scrambleNativePassword(password string, seed []byte) ([]byte, error) {
	if password == "" {
		return []byte{}, nil
	}
	h1 := sha1.Sum([]byte(password))
	h2 := sha1.Sum(h1[:])
	all := make([]byte, len(seed)+len(h2))
	copy(all, seed)
	copy(all[len(seed):], h2[:])
	h3 := sha1.Sum(all)

	out := make([]byte, len(h1))
	for i := range h1 {
		out[i] = h1[i] ^ h3[i]
	}
	return out, nil
}

const (
	okHeader       = 0x00
	errHeader      = 0xff
	moreDataHeader = 0x01

	comQuit = 0x01
	comPing = 0x0e
)

const (
	clientLongPassword     = 0x00000001
	clientLongFlag         = 0x00000004
	clientConnectWithDB    = 0x00000008
	clientProtocol41       = 0x00000200
	clientSecureConnection = 0x00008000
	clientPluginAuth       = 0x00080000
	clientDeprecateEOF     = 0x01000000
)

func readPacket(r io.Reader) ([]byte, byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, 0, err
	}
	length := int(uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16)
	seq := header[3]
	if length == 0 {
		return []byte{}, seq, nil
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, 0, err
	}
	return payload, seq, nil
}

func writePacket(w io.Writer, seq byte, payload []byte) error {
	length := len(payload)
	header := []byte{byte(length), byte(length >> 8), byte(length >> 16), seq}
	if _, err := w.Write(header); err != nil {
		return err
	}
	if length > 0 {
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

func parseError(packet []byte) error {
	if len(packet) < 3 {
		return errors.New("malformed error packet")
	}
	code := binary.LittleEndian.Uint16(packet[1:3])
	message := ""
	if len(packet) > 3 {
		offset := 3
		if packet[3] == '#' && len(packet) >= 9 {
			offset = 9
		}
		message = string(packet[offset:])
	}
	return fmt.Errorf("mysql error %d: %s", code, strings.TrimSpace(message))
}

// Result is a stub implementation so that ExecContext satisfies the driver.Result interface when
// operations are attempted. Since ExecContext always returns an error, this is unused but provided
// for completeness.
type Result struct{}

func (Result) LastInsertId() (int64, error) { return 0, errors.New("not supported") }
func (Result) RowsAffected() (int64, error) { return 0, errors.New("not supported") }
