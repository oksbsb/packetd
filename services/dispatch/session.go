package dispatch

import (
	"github.com/untangle/packetd/services/dict"
	"github.com/untangle/packetd/services/logger"
	"sync"
	"time"
)

// SessionEntry stores details related to a session
type SessionEntry struct {
	SessionID          uint64
	ConntrackID        uint32
	PacketCount        uint64
	ByteCount          uint64
	CreationTime       time.Time
	LastActivityTime   time.Time
	ClientSideTuple    Tuple
	ServerSideTuple    Tuple
	ConntrackConfirmed bool
	EventCount         uint64
	subscriptions      map[string]SubscriptionHolder
	subLocker          sync.Mutex
	attachments        map[string]interface{}
	attLocker          sync.Mutex
}

var sessionTable map[string]*SessionEntry
var sessionMutex sync.Mutex
var sessionIndex uint64

// nextSessionID returns the next sequential session ID value
func nextSessionID() uint64 {
	var value uint64
	sessionMutex.Lock()
	value = sessionIndex
	sessionIndex++

	if sessionIndex == 0 {
		sessionIndex++
	}

	sessionMutex.Unlock()
	return (value)
}

// findSessionEntry searches for an entry in the session table
func findSessionEntry(finder string) (*SessionEntry, bool) {
	sessionMutex.Lock()
	entry, status := sessionTable[finder]
	logger.Trace("Lookup session index %s -> %v\n", finder, status)
	sessionMutex.Unlock()
	return entry, status
}

// insertSessionEntry adds an entry to the session table
func insertSessionEntry(finder string, entry *SessionEntry) {
	logger.Trace("Insert session index %s -> %v\n", finder, entry.ClientSideTuple)
	sessionMutex.Lock()
	sessionTable[finder] = entry
	dict.AddSessionEntry(entry.ConntrackID, "session_id", entry.SessionID)
	sessionMutex.Unlock()
}

// removeSessionEntry removes an entry from the session table
func removeSessionEntry(finder string) {
	logger.Trace("Remove session index %s\n", finder)
	sessionMutex.Lock()
	entry, status := sessionTable[finder]
	if status {
		dict.DeleteSession(entry.ConntrackID)
	}
	delete(sessionTable, finder)
	sessionMutex.Unlock()
}

// PutSessionAttachment is used to safely add an attachment to a session object
func PutSessionAttachment(entry *SessionEntry, name string, value interface{}) {
	entry.attLocker.Lock()
	entry.attachments[name] = value
	entry.attLocker.Unlock()
}

// GetSessionAttachment is used to safely get an attachment from a session object
func GetSessionAttachment(entry *SessionEntry, name string) interface{} {
	entry.attLocker.Lock()
	value := entry.attachments[name]
	entry.attLocker.Unlock()
	return value
}

// cleanSessionTable cleans the session table by removing stale entries
func cleanSessionTable() {
	nowtime := time.Now()

	for key, val := range sessionTable {
		if (nowtime.Unix() - val.LastActivityTime.Unix()) < 600 {
			continue
		}
		removeSessionEntry(key)
		// This happens in some corner cases
		// such as a session that is blocked we will have a session in the session table
		// but it will never reach the conntrack confirmed state, and thus never
		// get a conntrack new or destroy event
		// as such this will exist in the table until the conntrack ID gets re-used
		// or this happens. Since this is condition is expected, just log as debug
		logger.Debug("Removing stale session entry %s %v\n", key, val.ClientSideTuple)
	}
}

// printSessionTable prints the session table
func printSessionTable() {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	for k, v := range sessionTable {
		logger.Debug("Session[%s] = %s\n", k, v.ClientSideTuple.String())
	}
}
