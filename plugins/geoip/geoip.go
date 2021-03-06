package geoip

import (
	"compress/gzip"
	"github.com/oschwald/geoip2-golang"
	"github.com/untangle/packetd/services/dict"
	"github.com/untangle/packetd/services/dispatch"
	"github.com/untangle/packetd/services/logger"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
)

const pluginName = "geoip"

var geodb *geoip2.Reader

// PluginStartup is called to allow plugin specific initialization.
// We initialize an instance of the GeoIP engine using any existing
// database we can find, or we download if needed. We increment the
// argumented WaitGroup so the main process can wait for our shutdown function
// to return during shutdown.
func PluginStartup() {
	logger.Info("PluginStartup(%s) has been called\n", pluginName)

	var filename string

	// start by looking for the NGFW city database file
	db, err := geoip2.Open(findGeoFile(true))
	if err != nil {
		logger.Warn("Unable to load GeoIP Database: %s\n", err)
	} else {
		logger.Info("Loading GeoIP Database: %s\n", filename)
		geodb = db
	}

	dispatch.InsertNfqueueSubscription(pluginName, 2, PluginNfqueueHandler)
}

// PluginShutdown is called when the daemon is shutting down. We close our
// GeoIP engine and call done for the argumented WaitGroup to let the main
// process know we're finished.
func PluginShutdown() {
	logger.Info("PluginShutdown(%s) has been called\n", pluginName)
	if geodb != nil {
		geodb.Close()
	}
}

// PluginNfqueueHandler is called to handle nfqueue packet data. We extract
// the source and destination IP address from the packet, lookup the GeoIP
// country code for each, and store them in the conntrack dictionary.
func PluginNfqueueHandler(mess dispatch.NfqueueMessage, ctid uint32, newSession bool) dispatch.NfqueueResult {
	var result dispatch.NfqueueResult
	result.Owner = pluginName
	result.PacketMark = 0
	result.SessionRelease = true

	// release immediately as we only care about the first packet
	dispatch.ReleaseSession(mess.Session, pluginName)
	if !newSession {
		return result
	}

	if geodb == nil {
		return result
	}

	var srcAddr net.IP
	var dstAddr net.IP
	var SrcCode = "XX"
	var DstCode = "XX"

	if mess.IP6layer != nil {
		srcAddr = mess.IP6layer.SrcIP
		dstAddr = mess.IP6layer.DstIP
	}

	if mess.IP4layer != nil {
		srcAddr = mess.IP4layer.SrcIP
		dstAddr = mess.IP4layer.DstIP
	}

	SrcRecord, err := geodb.City(srcAddr)
	if (err == nil) && (len(SrcRecord.Country.IsoCode) != 0) {
		SrcCode = SrcRecord.Country.IsoCode
	}

	DstRecord, err := geodb.City(dstAddr)
	if (err == nil) && (len(DstRecord.Country.IsoCode) != 0) {
		DstCode = DstRecord.Country.IsoCode
	}

	logger.Debug("SRC: %v = %s\n", srcAddr, SrcCode)
	logger.Debug("DST: %v = %s\n", dstAddr, DstCode)

	dict.AddSessionEntry(ctid, "client_country", SrcCode)
	dict.AddSessionEntry(ctid, "server_country", DstCode)

	return result
}

func databaseDownload(filename string) {
	logger.Info("Downloading GeoIP Database...\n")

	// Make sure the target directory exists
	marker := strings.LastIndex(filename, "/")

	// Get the index of the last slash so we can isolate the path and create the directory
	if marker > 0 {
		os.MkdirAll(filename[0:marker], 0755)
	}

	// Get the GeoIP database from MaxMind
	resp, err := http.Get("http://geolite.maxmind.com/download/geoip/database/GeoLite2-Country.mmdb.gz")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		logger.Warn("Download failure: %s\n", resp.Status)
		return
	}

	// Create a reader for the compressed data
	reader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return
	}
	defer reader.Close()

	// Create the output file
	writer, err := os.Create(filename)
	if err != nil {
		return
	}
	defer writer.Close()

	// Write the uncompressed database to the file
	io.Copy(writer, reader)
	logger.Info("Downloaded  GeoIP Database.\n")
}

// findGeoFile finds the location of the GeoLite2-City.mmdb file
// it checks several common locations and returns any found
// if none found it just returns "/tmp/GeoLite2-City.mmdb"
func findGeoFile(download bool) string {
	possibleLocations := []string{
		"/var/cache/untangle-geoip/GeoLite2-City.mmdb",
		"/tmp/GeoLite2-City.mmdb",
		"/usr/lib/GeoLite2-City.mmdb",
		"/usr/share/untangle-geoip/GeoLite2-Country.mmdb",
		"/usr/share/geoip/GeoLite2-Country.mmdb",
	}

	for _, filename := range possibleLocations {
		_, err := os.Stat(filename)
		if os.IsNotExist(err) {
			continue
		} else {
			return filename
		}
	}

	// If we reach this point it was not found
	if download {
		databaseDownload("/usr/lib/GeoLite2-City.mmdb")
		// try again now that we tried to download
		// but do not download again
		return findGeoFile(false)
	}

	// Not found - just return one
	return "/tmp/GeoLite2-City.mmdb"
}
