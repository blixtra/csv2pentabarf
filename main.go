package main

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/csv"
	"encoding/xml"
	"fmt"
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
	Subtitle         string   `xml:"subtitle"`
	Venue            string   `xml:"venue"`
	City             string   `xml:"city"`
	Start            string   `xml:"start"`
	End              string   `xml:"end"`
	Days             int      `xml:"days"`
	DayChange        string   `xml:"day_change"`
	TimeslotDuration string   `xml:"timeslot_duration"`
}

type Person struct {
	XMLName xml.Name `xml:"person"`
	Id      int      `xml:"id,attr"`
	Name    string   `xml:",chardata"`
}

type Event struct {
	XMLName     xml.Name `xml:"event"`
	Id          int      `xml:"id,attr"`
	Start       string   `xml:"start"`
	Duration    string   `xml:"duration"`
	Room        string   `xml:"room"`
	Slug        string   `xml:"slug"`
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
	conf   = &Conference{Title: "systemd.conf 2015", Venue: "betahaus",
		City: "Berlin", Start: "2015-11-05", End: "2015-11-07", Days: 2,
		DayChange: "08:00:00", TimeslotDuration: "00:05:00"}
)

func main() {
	//d := &Day{Index: 1, Date: "2015-11-04", Rooms: []Room{*r}}

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
		e := &Event{Id: id, Start: t, Duration: dur, Room: "Main room",
			Title: r[idx[""]], Type: "Talk", Description: r[idx["description"]],
			Persons: ppl}
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
		cd = append(cd, Day{Index: i, Date: d, Rooms: []Room{r}})
	}

	schedule := Schedule{Conf: *conf, Days: cd}
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

func GenerateSlug(str string) (slug string) {
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
	return int(math.Mod(float64(b+uint64(l)), 1024))
}
