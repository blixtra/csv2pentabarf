package main

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Pentebarf XML description
type Conference struct {
	XMLName          xml.Name `xml:"conference"`
	Title            string   `xml:"title"`
	Acronym          string   `xml:"acronym"`
	Start            string   `xml:"start"`
	End              string   `xml:"end"`
	Days             int      `xml:"days"`
	TimeslotDuration string   `xml:"timeslot_duration"`
}

type Person struct {
	XMLName xml.Name `xml:"person"`
	Id      int      `xml:"id,attr"`
	Name    string   `xml:",chardata"`
}

type Recording struct {
	XMLName xml.Name `xml:"recording"`
	License string   `xml:"license"`
	Optout  string   `xml:"optout"`
}

type Event struct {
	XMLName     xml.Name `xml:"event"`
	Guid        string   `xml:"guid,attr"`
	Id          int      `xml:"id,attr"`
	Date        string   `xml:"date"`
	Start       string   `xml:"start"`
	Duration    string   `xml:"duration"`
	Room        string   `xml:"room"`
	Slug        string   `xml:"slug"`
	Rec         Recording
	Title       string   `xml:"title"`
	Subtitle    string   `xml:"subtitle"`
	Track       string   `xml:"track"`
	Type        string   `xml:"type"`
	Language    string   `xml:"language"`
	Abstract    string   `xml:"abstract"`
	Description string   `xml:"description"`
	Persons     []Person `xml:"persons>person"`
	Links       string   `xml:"links"`
}

type Events []*Event

func (e Events) Len() int           { return len(e) }
func (e Events) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
func (e Events) Less(i, j int) bool { return e[i].Start < e[j].Start }

type Room struct {
	XMLName xml.Name `xml:"room"`
	Name    string   `xml:"name,attr"`
	Events  Events   `xml:"events"`
}

type Day struct {
	XMLName xml.Name `xml:"day"`
	Index   int      `xml:"index,attr"`
	Date    string   `xml:"date,attr"`
	Rooms   []Room   `xml:rooms`
}

type Schedule struct {
	XMLName xml.Name `xml:"schedule"`
	Version string   `xml:"version"`
	Conf    Conference
	Days    []Day `xml:"days"`
}

// CSV data
var (
	idx = map[string]int{
		"eId":         0,
		"start":       1,
		"end":         2,
		"title":       3,
		"description": 4,
		"speakers":    5,
		"org":         6,
	}
	header = `<?xml version="1.0" encoding="UTF-8"?>`
	conf   = &Conference{Title: "systemd.conf 2015", Acronym: "systemdconf2015",
		Start: "2015-11-05", End: "2015-11-07", Days: 2, TimeslotDuration: "00:05:00"}
	recording = &Recording{License: "CC-BY-SA", Optout: "false"}
)

func main() {
	fmt.Println("\nReading CSV")
	csvFile, err := os.Open("schedule.csv")
	if err != nil {
		panic(err)
	}

	csvr := csv.NewReader(csvFile)
	csvr.Comma = ','
	csvr.Comment = '#'

	records, err := csvr.ReadAll()
	if err != nil {
		fmt.Printf("error with csv: %v\n", err)
	}

	fmt.Println("\nProcessing CSV")
	days := make(map[string]Events)
	for _, r := range records {
		// Prepare
		d, t, dur := getTimeInfo(r[idx["start"]], r[idx["end"]])
		var ppl []Person
		spkrs := strings.Split(r[idx["speakers"]], ",")
		for _, spkr := range spkrs {
			ppl = append(ppl, Person{Id: genSpeakerId(spkr), Name: spkr})
		}
		var id int
		if id, err = strconv.Atoi(r[idx["eId"]]); err != nil {
			panic(fmt.Sprintf("Couldn't convert %v to int", r[idx["eId"]]))
		}
		//e := &Event{Id: 2, Start: "12:00", Duration: "00:30", Room: "Main",
		//Title: "A Title", Slug: "a_title", Track: "Containers", Type: "Talk",
		//Abstract: "An abstract", Description: "A Longer Description",
		//Persons: []Person{*p}}
		uuid, _ := newUUID()
		e := &Event{Guid: uuid, Id: id, Date: fmt.Sprintf("%sT%s:00+01:00", d, t),
			Start: t, Duration: dur, Room: "Main room", Slug: genSlug(r[idx["title"]]),
			Rec: *recording, Title: r[idx["title"]], Type: "Talk", Track: "Main",
			Language: "en", Abstract: r[idx["description"]], Persons: ppl}
		days[d] = append(days[d], e)
	}

	var sd []string
	for d, events := range days {
		sort.Sort(events)
		sd = append(sd, d)
	}
	sort.Strings(sd)

	var cd []Day
	for i, d := range sd {
		r := Room{Name: "Main room", Events: days[d]}
		cd = append(cd, Day{Index: i + 1, Date: d, Rooms: []Room{r}})
	}

	schedule := Schedule{Conf: *conf, Days: cd, Version: "1"}
	output, err := xml.MarshalIndent(schedule, "  ", "  ")
	if err != nil {
		fmt.Printf("error with XML: %v\n", err)
	}

	fmt.Println("\nPreparing to write XML file")
	f, err := os.Create("schedule.xml")
	if err != nil {
		panic(fmt.Sprintf("Can't create schedule.xml in the current folder: %e", err))
	}
	defer f.Close()

	if _, err := f.Write(output); err != nil {
		panic(fmt.Sprintf("Problem writing: %v", err))
	}
	fmt.Println("\nSuccess!")
}

func getTimeInfo(start string, end string) (d string, t string, dur string) {
	// Format: 11/5/15 11:25
	tFmt := "01/2/06 15:04"
	var s, e time.Time
	var err error
	if s, err = time.Parse(tFmt, start); err != nil {
		fmt.Printf("Could not parse start time: %v", err)
	}
	if e, err = time.Parse(tFmt, end); err != nil {
		fmt.Printf("Could not parse end time: %v", err)
	}

	dFmt := "2006-01-02"
	d = s.Format(dFmt)
	tFmt = "15:04"
	t = s.Format(tFmt)
	diff := e.Sub(s)
	dur = fmt.Sprintf("00:%2.f", diff.Minutes())
	return d, t, dur
}

func genSlug(str string) (slug string) {
	return strings.Map(func(r rune) rune {
		switch {
		case r == ' ', r == '-':
			return '-'
		case r == '_', unicode.IsLetter(r), unicode.IsDigit(r):
			return r
		default:
			return -1
		}
		return -1
	}, strings.ToLower(strings.TrimSpace(str)))
}

func genSpeakerId(s string) int {
	m := md5.Sum([]byte(s))
	b, l := binary.Uvarint(m[:])
	return int(math.Mod(float64(b+uint64(l)), 1024)) + 1
}

// newUUID generates a random UUID according to RFC 4122
func newUUID() (string, error) {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		return "", err

	}
	// variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), nil
}
