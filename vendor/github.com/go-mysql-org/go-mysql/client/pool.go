package client

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/pingcap/errors"
)

/*
Pool for efficient reuse of connections.

Usage:
	pool := client.NewPool(log.Debugf, 100, 400, 5, `127.0.0.1:3306`, `username`, `userpwd`, `dbname`)
	...
	conn, _ := pool.GetConn(ctx)
	defer pool.PutConn(conn)
	conn.Execute/conn.Begin/etc...
*/

type (
	Timestamp int64

	LogFunc func(format string, args ...interface{})

	Pool struct {
		logFunc          LogFunc
		minAlive         int
		maxAlive         int
		maxIdle          int
		idleCloseTimeout Timestamp
		idlePingTimeout  Timestamp
		connect          func() (*Conn, error)

		synchro struct {
			sync.Mutex
			idleConnections []Connection
			stats           ConnectionStats
		}

		readyConnection chan Connection
	}

	ConnectionStats struct {
		// Uses internally
		TotalCount int

		// Only for stats
		IdleCount    int
		CreatedCount int64
	}

	Connection struct {
		conn      *Conn
		lastUseAt Timestamp
	}
)

var (
	// MaxIdleTimeoutWithoutPing - If the connection has been idle for more than this time,
	//   then ping will be performed before use to check if it alive
	MaxIdleTimeoutWithoutPing = 10 * time.Second

	// DefaultIdleTimeout - If the connection has been idle for more than this time,
	//   we can close it (but we should remember about Pool.minAlive)
	DefaultIdleTimeout = 30 * time.Second

	// MaxNewConnectionAtOnce - If we need to create new connections,
	//   then we will create no more than this number of connections at a time.
	// This restriction will be ignored on pool initialization.
	MaxNewConnectionAtOnce = 5
)

// NewPool initializes new connection pool and uses params: addr, user, password, dbName and options.
//     minAlive specifies the minimum number of open connections that the pool will try to maintain.
//     maxAlive specifies the maximum number of open connections
//       (for internal reasons, may be greater by 1 inside newConnectionProducer).
//     maxIdle specifies the maximum number of idle connections (see DefaultIdleTimeout).
func NewPool(
	logFunc LogFunc,
	minAlive int,
	maxAlive int,
	maxIdle int,
	addr string,
	user string,
	password string,
	dbName string,
	options ...func(conn *Conn),
) *Pool {
	if minAlive > maxAlive {
		minAlive = maxAlive
	}
	if maxIdle > maxAlive {
		maxIdle = maxAlive
	}
	if maxIdle <= minAlive {
		maxIdle = minAlive
	}

	pool := &Pool{
		logFunc:  logFunc,
		minAlive: minAlive,
		maxAlive: maxAlive,
		maxIdle:  maxIdle,

		idleCloseTimeout: Timestamp(math.Ceil(DefaultIdleTimeout.Seconds())),
		idlePingTimeout:  Timestamp(math.Ceil(MaxIdleTimeoutWithoutPing.Seconds())),

		connect: func() (*Conn, error) {
			return Connect(addr, user, password, dbName, options...)
		},

		readyConnection: make(chan Connection),
	}

	pool.synchro.idleConnections = make([]Connection, 0, pool.maxIdle)

	go pool.newConnectionProducer()

	if pool.minAlive > 0 {
		pool.logFunc(`Pool: Setup %d new connections (minimal pool size)...`, pool.minAlive)
		pool.startNewConnections(pool.minAlive)
	}

	go pool.closeOldIdleConnections()

	return pool
}

func (pool *Pool) GetStats(stats *ConnectionStats) {
	pool.synchro.Lock()

	*stats = pool.synchro.stats

	stats.IdleCount = len(pool.synchro.idleConnections)

	pool.synchro.Unlock()
}

// GetConn returns connection from the pool or create new
func (pool *Pool) GetConn(ctx context.Context) (*Conn, error) {
	for {
		connection, err := pool.getConnection(ctx)
		if err != nil {
			return nil, err
		}

		// For long time idle connections, we do a ping check
		if delta := pool.nowTs() - connection.lastUseAt; delta > pool.idlePingTimeout {
			if err := pool.ping(connection.conn); err != nil {
				pool.closeConn(connection.conn)
				continue
			}
		}

		return connection.conn, nil
	}
}

// PutConn returns working connection back to pool
func (pool *Pool) PutConn(conn *Conn) {
	pool.putConnection(Connection{
		conn:      conn,
		lastUseAt: pool.nowTs(),
	})
}

// DropConn closes the connection without any checks
func (pool *Pool) DropConn(conn *Conn) {
	pool.closeConn(conn)
}

func (pool *Pool) putConnection(connection Connection) {
	pool.synchro.Lock()
	defer pool.synchro.Unlock()

	// If someone is already waiting for a connection, then we return it to him
	select {
	case pool.readyConnection <- connection:
		return
	default:
	}

	// Nobody needs this connection

	pool.putConnectionUnsafe(connection)
}

func (pool *Pool) nowTs() Timestamp {
	return Timestamp(time.Now().Unix())
}

func (pool *Pool) getConnection(ctx context.Context) (Connection, error) {
	pool.synchro.Lock()

	connection := pool.getIdleConnectionUnsafe()
	if connection.conn != nil {
		pool.synchro.Unlock()
		return connection, nil
	}
	pool.synchro.Unlock()

	// No idle connections are available

	select {
	case connection := <-pool.readyConnection:
		return connection, nil

	case <-ctx.Done():
		return Connection{}, ctx.Err()
	}
}

func (pool *Pool) putConnectionUnsafe(connection Connection) {
	if len(pool.synchro.idleConnections) == cap(pool.synchro.idleConnections) {
		pool.synchro.stats.TotalCount--
		_ = connection.conn.Close() // Could it be more effective to close older connections?
	} else {
		pool.synchro.idleConnections = append(pool.synchro.idleConnections, connection)
	}
}

func (pool *Pool) newConnectionProducer() {
	var connection Connection
	var err error

	for {
		connection.conn = nil

		pool.synchro.Lock()

		connection = pool.getIdleConnectionUnsafe()
		if connection.conn == nil {
			if pool.synchro.stats.TotalCount >= pool.maxAlive {
				// Can't create more connections
				pool.synchro.Unlock()
				time.Sleep(10 * time.Millisecond)
				continue
			}
			pool.synchro.stats.TotalCount++ // "Reserving" new connection
		}

		pool.synchro.Unlock()

		if connection.conn == nil {
			connection, err = pool.createNewConnection()
			if err != nil {
				pool.synchro.Lock()
				pool.synchro.stats.TotalCount-- // Bad luck, should try again
				pool.synchro.Unlock()

				time.Sleep(time.Duration(10+rand.Intn(90)) * time.Millisecond)
				continue
			}
		}

		pool.readyConnection <- connection
	}
}

func (pool *Pool) createNewConnection() (Connection, error) {
	var connection Connection
	var err error

	connection.conn, err = pool.connect()
	if err != nil {
		return Connection{}, errors.Errorf(`Could not connect to mysql: %s`, err)
	}
	connection.lastUseAt = pool.nowTs()

	pool.synchro.Lock()
	pool.synchro.stats.CreatedCount++
	pool.synchro.Unlock()

	return connection, nil
}

func (pool *Pool) getIdleConnectionUnsafe() Connection {
	cnt := len(pool.synchro.idleConnections)
	if cnt == 0 {
		return Connection{}
	}

	last := cnt - 1
	connection := pool.synchro.idleConnections[last]
	pool.synchro.idleConnections[last].conn = nil
	pool.synchro.idleConnections = pool.synchro.idleConnections[:last]

	return connection
}

func (pool *Pool) closeOldIdleConnections() {
	var toPing []Connection

	ticker := time.NewTicker(5 * time.Second)

	for range ticker.C {
		toPing = pool.getOldIdleConnections(toPing[:0])
		if len(toPing) == 0 {
			continue
		}
		pool.recheckConnections(toPing)

		if !pool.spawnConnectionsIfNeeded() {
			pool.closeIdleConnectionsIfCan()
		}
	}
}

func (pool *Pool) getOldIdleConnections(dst []Connection) []Connection {
	dst = dst[:0]

	pool.synchro.Lock()

	synchro := &pool.synchro

	idleCnt := len(synchro.idleConnections)
	checkBefore := pool.nowTs() - pool.idlePingTimeout

	for i := idleCnt - 1; i >= 0; i-- {
		if synchro.idleConnections[i].lastUseAt > checkBefore {
			continue
		}

		dst = append(dst, synchro.idleConnections[i])

		last := idleCnt - 1
		if i < last {
			// Removing an item from the middle of a slice
			synchro.idleConnections[i], synchro.idleConnections[last] = synchro.idleConnections[last], synchro.idleConnections[i]
		}

		synchro.idleConnections[last].conn = nil
		synchro.idleConnections = synchro.idleConnections[:last]
		idleCnt--
	}

	pool.synchro.Unlock()

	return dst
}

func (pool *Pool) recheckConnections(connections []Connection) {
	const workerCnt = 2 // Heuristic :)

	queue := make(chan Connection, len(connections))
	for _, connection := range connections {
		queue <- connection
	}
	close(queue)

	var wg sync.WaitGroup
	wg.Add(workerCnt)
	for worker := 0; worker < workerCnt; worker++ {
		go func() {
			defer wg.Done()
			for connection := range queue {
				if err := pool.ping(connection.conn); err != nil {
					pool.closeConn(connection.conn)
				} else {
					pool.putConnection(connection)
				}
			}
		}()
	}

	wg.Wait()
}

// spawnConnectionsIfNeeded creates new connections if there are not enough of them and returns true in this case
func (pool *Pool) spawnConnectionsIfNeeded() bool {
	pool.synchro.Lock()
	totalCount := pool.synchro.stats.TotalCount
	idleCount := len(pool.synchro.idleConnections)
	needSpanNew := pool.minAlive - totalCount
	pool.synchro.Unlock()

	if needSpanNew <= 0 {
		return false
	}

	// Не хватает соединений, нужно создать еще

	if needSpanNew > MaxNewConnectionAtOnce {
		needSpanNew = MaxNewConnectionAtOnce
	}

	pool.logFunc(`Pool: Setup %d new connections (total: %d idle: %d)...`, needSpanNew, totalCount, idleCount)
	pool.startNewConnections(needSpanNew)

	return true
}

func (pool *Pool) closeIdleConnectionsIfCan() {
	pool.synchro.Lock()

	canCloseCnt := pool.synchro.stats.TotalCount - pool.minAlive
	canCloseCnt-- // -1 to account for an open but unused connection (pool.readyConnection <- connection in newConnectionProducer)

	idleCnt := len(pool.synchro.idleConnections)

	inFly := pool.synchro.stats.TotalCount - idleCnt

	// We can close no more than 10% connections at a time, but at least 1, if possible
	idleCanCloseCnt := idleCnt / 10
	if idleCanCloseCnt == 0 {
		idleCanCloseCnt = 1
	}
	if canCloseCnt > idleCanCloseCnt {
		canCloseCnt = idleCanCloseCnt
	}
	if canCloseCnt <= 0 {
		pool.synchro.Unlock()
		return
	}

	closeFromIdx := idleCnt - canCloseCnt
	if closeFromIdx < 0 {
		// If there are enough requests in the "flight" now, then we can close all unnecessary
		closeFromIdx = 0
	}

	toClose := append([]Connection{}, pool.synchro.idleConnections[closeFromIdx:]...)

	for i := closeFromIdx; i < idleCnt; i++ {
		pool.synchro.idleConnections[i].conn = nil
	}
	pool.synchro.idleConnections = pool.synchro.idleConnections[:closeFromIdx]

	pool.synchro.Unlock()

	pool.logFunc(`Pool: Close %d idle connections (in fly %d)`, len(toClose), inFly)
	for _, connection := range toClose {
		pool.closeConn(connection.conn)
	}
}

func (pool *Pool) closeConn(conn *Conn) {
	pool.synchro.Lock()
	pool.synchro.stats.TotalCount--
	pool.synchro.Unlock()

	_ = conn.Close() // Closing is not an instant action, so do it outside the lock
}

func (pool *Pool) startNewConnections(count int) {
	connections := make([]Connection, 0, count)
	for i := 0; i < count; i++ {
		if conn, err := pool.createNewConnection(); err == nil {
			pool.synchro.Lock()
			pool.synchro.stats.TotalCount++
			pool.synchro.Unlock()
			connections = append(connections, conn)
		}
	}

	pool.synchro.Lock()
	for _, connection := range connections {
		pool.putConnectionUnsafe(connection)
	}
	pool.synchro.Unlock()
}

func (pool *Pool) ping(conn *Conn) error {
	deadline := time.Now().Add(100 * time.Millisecond)
	_ = conn.SetWriteDeadline(deadline)
	_ = conn.SetReadDeadline(deadline)
	err := conn.Ping()
	if err != nil {
		pool.logFunc(`Pool: ping query fail: %s`, err.Error())
	}
	return err
}
