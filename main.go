package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/jiguorui/crc16"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	channelListDirectory := flag.String("dir", "/Users/mustafa/PhilipsChannelMaps/ChannelMap_45/ChannelList", "ChannelList directory location")
	listFile := flag.String("list", "list.json", "JSON channel list file")
	flag.Parse()

	// calculate db crc16
	data, err := ioutil.ReadFile(fmt.Sprintf("%s/tv.db", *channelListDirectory))
	if err != nil {
		log.Fatalln(err)
	}
	oldDBCrc := fmt.Sprintf("%x", crc16.CheckSum(data))
	oldDBCrcReverse := fmt.Sprintf("%s%s", oldDBCrc[2:4], oldDBCrc[0:2])

	// get desired channel order from json file
	js, err := ioutil.ReadFile(*listFile)
	if err != nil {
		log.Fatalln(err)
	}
	var channelOrders []ChannelOrder
	err = json.Unmarshal(js, &channelOrders)
	if err != nil {
		log.Fatalln(err)
	}
	channelOrderMap := make(map[string]int)
	for _, o := range channelOrders {
		channelOrderMap[o.Name] = o.Order
	}

	// connect to db
	db, err := sqlx.Connect("sqlite3", fmt.Sprintf("%s/tv.db", *channelListDirectory))
	if err != nil {
		log.Fatalln(err)
	}
	// get channels
	channels := []Channel{}
	db.Select(&channels, "SELECT _id, type, service_type, display_number, display_name FROM channels WHERE type='TYPE_DVB_S2' ORDER BY display_number ASC")
	var unknownTvIndex = 200
	var unknownRadioIndex = 1000
	var unknownIndex = 1500
	updateMap := make(map[int]int)
	for _, channel := range channels {
		if val, ok := channelOrderMap[channel.Name]; ok {
			_, hasAdded := updateMap[val]
			if !hasAdded {
				updateMap[val] = channel.Id
				continue
			}
		}
		if channel.ServiceType == "SERVICE_TYPE_AUDIO_VIDEO" {
			updateMap[unknownTvIndex] = channel.Id
			unknownTvIndex++
		} else if channel.ServiceType == "SERVICE_TYPE_AUDIO" {
			updateMap[unknownRadioIndex] = channel.Id
			unknownRadioIndex++
		} else {
			updateMap[unknownIndex] = channel.Id
			unknownIndex++
		}
	}

	for order, id := range updateMap {
		sqlStatement := `UPDATE channels SET display_number=? WHERE _id=?;`
		_, err := db.Exec(sqlStatement, fmt.Sprintf("%d", order), id)
		if err != nil {
			log.Fatalln(err)
		}
	}
	// close db
	db.Close()

	// calculate new crc16 for db
	data, err = ioutil.ReadFile(fmt.Sprintf("%s/tv.db", *channelListDirectory))
	if err != nil {
		log.Fatalln(err)
	}
	newDBCrc := fmt.Sprintf("%x", crc16.CheckSum(data))
	newDBCrcReverse := fmt.Sprintf("%s%s", newDBCrc[2:4], newDBCrc[0:2])
	lstData, err := ioutil.ReadFile(fmt.Sprintf("%s/chanLst.bin", *channelListDirectory))
	if err != nil {
		log.Fatalln(err)
	}
	decodedOld, err := hex.DecodeString(oldDBCrcReverse)
	if err != nil {
		log.Fatalln(err)
	}
	decodedNew, err := hex.DecodeString(newDBCrcReverse)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("Old crc:%s new crc: %s\n", oldDBCrcReverse, newDBCrcReverse)

	newFile := bytes.Replace(lstData, []byte(decodedOld), []byte(decodedNew), -1)
	err = ioutil.WriteFile(fmt.Sprintf("%s/chanLst.bin", *channelListDirectory), newFile, 0644)
	if err != nil {
		log.Fatalln(err)
	}

	var files []string
	err = filepath.Walk(*channelListDirectory, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		log.Fatalln(err)
	}
	t := time.Now()
	for _, f := range files {
		if err := os.Chtimes(f, t, t); err != nil {
			log.Fatalln(err)
		}
	}
	t = time.Now().Add(time.Second)
	if err := os.Chtimes(fmt.Sprintf("%s/chanLst.bin", *channelListDirectory), t, t); err != nil {
		log.Fatalln(err)
	}
	log.Println("Channels updated.")
}

type Channel struct {
	Id          int    `db:"_id"`
	Type        string `db:"type"`
	ServiceType string `db:"service_type"`
	Order       string `db:"display_number"`
	Name        string `db:"display_name"`
}

type ChannelOrder struct {
	Name  string `json:"Name"`
	Order int    `json:"Order"`
}
