package steaminviter

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"io/ioutil"
	"encoding/json"
	"time"
	"regexp"
	"errors"
	"database/sql"
	"github.com/go-sql-driver/mysql"
	"log"
)

const baseid int64	= 76561198267374397			// Base ID MUST not be a private profile
const apikey string	= "2B2A0C37AC20B5DC2234E579A2ABB11C"
var errNum int

func main() {
	dbUsers	:= make([]int64, 0, 10000000)
	users	:= make([]int64, 0, 1000000)
	nextID	:= baseid
	var (
		totalScanned	int
		latestUpdated	int
		totalAdded	int
		databaseTotal	int
		duplicates	int
		lastInserted	int64
		queryString	string
		err		error
	)

	startTime := makeTimestamp()
	defer func(){
		recover()
		fmt.Println("Time Elapsed (ms):", makeTimestamp() - startTime)
		fmt.Println("Last Inserted ID:", strconv.FormatInt(lastInserted, 10))
	}()

	users = append(users, baseid)

	if ids, err := getFriendsList(nextID); err != nil {
		fmt.Println(err)
	} else {
		totalScanned++
		for _, id := range ids {
			if !existsInSlice(users, id) {
				users = append(users, id)
			}
		}
	}

	db, err := sql.Open("mysql", "steaminviter:AriisAwesome9@tcp(45.32.189.171:3306)/steaminviter")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		panic(err)
	}

	var steamid int64

	rows, err := db.Query("select steamid from steamids")
	if err != nil {
		panic(err)
	}

	for rows.Next() {
		err := rows.Scan(&steamid)
		if err != nil {
			panic(err)
		}
		dbUsers = append(dbUsers, steamid)
		databaseTotal++
	}
	err = rows.Err()
	if err != nil {
		panic(err)
	}

	rows.Close()

	_ = mysql.Config{}

	go func(){
		for {
			duplicates = 0
			for i := 0; i < 80; i++ {
				ids, _ := getFriendsList(users[totalScanned])
				for _, id := range ids {
					if !existsInSlice(users, id) {
						users = append(users, id)
					}
				}
				totalScanned++
			}

			if users[totalScanned] == lastInserted {
				fmt.Println("No new users were found... continuing!")
				continue
			}

			totalAdded = 0

			queryString = "INSERT INTO steamids (steamid) VALUES "

			for i := latestUpdated; i < len(users); i++ {
				if !existsInSlice(dbUsers, users[i]) {
					totalAdded++
					databaseTotal++
					queryString += "(" + strconv.FormatInt(users[i], 10) + "), "
					lastInserted = users[i]
				} else {
					duplicates++
				}
				if databaseTotal >= 20000000 {
					break
				}
			}

			if queryString == "INSERT INTO steamids (steamid) VALUES " {
				fmt.Println("No new users were found... continuing!")
				continue
			}

			if err = db.Ping(); err != nil {
				db, err := sql.Open("mysql", "steaminviter:AriisAwesome9@tcp(45.32.189.171:3306)/steaminviter")
				if err != nil {
					errNum++
					checkErrNum()
				}

				if err = db.Ping(); err != nil {
					errNum++
					checkErrNum()
				}
			}

			rows, err := db.Query(queryString[:len(queryString)-2])
			if err != nil {
				fmt.Println(queryString)
				errNum++
				checkErrNum()
			}
			rows.Close()

			fmt.Println("================================================")
			fmt.Println("Total Users Scanned:", len(users))
			fmt.Println("Last Inserted ID:", strconv.FormatInt(lastInserted, 10))
			fmt.Println("Added", totalAdded, "new Steam ID's to Database.")
			fmt.Println("Duplicates", duplicates)
			fmt.Println("Total Database IDS", databaseTotal)
			fmt.Println("================================================")

			if databaseTotal >= 20000000 {
				log.Fatal("Collected 20,000,000 Steam IDs. Exiting.")
			}

			latestUpdated = len(users) + 1
		}
	}()

	fmt.Println("PRESS ANY KEY TO STOP SCRIPT!")
	fmt.Scanln()
}

func getFriendsList(steamid int64) ([]int64, error) {
	resp, err := http.Get("http://api.steampowered.com/ISteamUser/GetFriendList/v0001/?" + url.Values{
		"key":		[]string{apikey},
		"steamid":	[]string{strconv.FormatInt(steamid, 10)},
		"relationship":	[]string{"friend"},
	}.Encode())
	defer resp.Body.Close()
	if err != nil {
		return []int64{}, err
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []int64{}, err
	}

	r, err := regexp.Compile(`<title>500 Internal Server Error<\/title>`)
	if err != nil {
		return []int64{}, err
	}
	if r.Match(content) {
		return []int64{}, errors.New("Steam ID not found.")
	}

	var extracted map[string]interface{}

	if err := json.Unmarshal(content, &extracted); err != nil {
		return []int64{}, err
	}

	switch extracted["friendslist"].(type) {
	case nil:
		return []int64{}, errors.New("No friends found.")
	}

	friends := make([]int64, len(extracted["friendslist"].(map[string]interface{})["friends"].([]interface{})))

	for key, value := range extracted["friendslist"].(map[string]interface{})["friends"].([]interface{}) {
		friends[key], err = strconv.ParseInt(value.(map[string]interface{})["steamid"].(string), 10, 64)
		if err != nil {
			return []int64{}, err
		}
	}

	return friends, nil
}

func existsInSlice(slice []int64, search int64) bool {
	for _, v := range slice {
		if v == search {
			return true
		}
	}
	return false
}

func searchDuplicates(slice []int64) bool {
	for _, v := range slice {
		occurances := 0
		for _, x := range slice {
			if v == x {
				occurances++
			}
			if occurances > 1 {
				return true
			}
		}
	}
	return false
}

func checkErrNum() {
	if errNum > 10 {
		panic("Too many errors")
	}
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}