package main

import (
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/solovev/steam_go"
	"gopkg.in/gomail.v2"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"fmt"
	"os"
)

const apikey		string			= "2B2A0C37AC20B5DC2234E579A2ABB11C"
var store		*sessions.CookieStore	= sessions.NewCookieStore([]byte("secured-cookies"))
var steamWallpaper	string			= getSteamBackground()

type steamUser struct {
	steamid				int64
	personaname			string		// Steam Name
	profileurl			string
	communityvisibilitystate	int8		// 1 = Private, 3 = Public
	profilestate			int8		// 0 = Profile not setup, 1 = Profile setup
	lastlogoff			int
	commentpermission		int8
	avatar				string		// 32x32px avatar
	avatarmedium			string		// 64x64px avatar
	avatarfull			string		// 184x184px avatar
	personastate			int8		// 0 = Offline, 1 = Online, 2 = Busy, 3 = Away, 4 = Snooze, 5 = Looking to Trade, 6 = Looking to Play
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cssHandler := http.FileServer(http.Dir("./css/"))
	jsHandler := http.FileServer(http.Dir("./js/"))
	imagesHandler := http.FileServer(http.Dir("./images/"))

	http.Handle("/css/", http.StripPrefix("/css/", cssHandler))
	http.Handle("/js/", http.StripPrefix("/js/", jsHandler))
	http.Handle("/images/", http.StripPrefix("/images/", imagesHandler))

	http.HandleFunc("/", httpHandler("index.gohtml", map[string]interface{}{}))
	http.HandleFunc("/features", httpHandler("features.gohtml", map[string]interface{}{}))
	http.HandleFunc("/purchase", httpHandler("purchase.gohtml", map[string]interface{}{}))
	http.HandleFunc("/contact", httpHandler("contact.gohtml", map[string]interface{}{"js": []string{"/js/contact.js"}}))
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/sendMessage", sendMessageHandler)
	http.HandleFunc("/ipn", ipnHandler)

	log.Println("Listening to :8080")
	http.ListenAndServe(":8080", nil)
}

// Main Functions
func httpHandler(path string, variables map[string]interface{}) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defaultHandler(w, r, path, variables)
	}
}

func defaultHandler(w http.ResponseWriter, r *http.Request, path string, variables map[string]interface{}) {
	// Check if url is a profile
	if match, _ := regexp.MatchString(`^\d+$`, r.URL.Path[1:]); len(r.URL.Path[1:]) == 17 && match {
		profileHandler(w, r)
		return
	}

	// Define urlPath from path variable
	urlPath := "/" + strings.Split(path, ".")[0]

	// Check if path is index
	if urlPath == "/index" {
		urlPath = "/"
	}

	// Check if url is different than wanted
	if r.URL.Path != urlPath {
		errorHandler(w, r, http.StatusNotFound)
		return
	}

	// Define template variables
	parse			:=	make(map[string]interface{})
	parse["steamid"]	=	0

	// Collect session variables
	session, err := store.Get(r, "user")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Parse user variables to template
	if !session.IsNew {
		parse["steamid"]	= session.Values["steamid"].(int64)
		parse["personaname"]	= session.Values["personaname"].(string)
		parse["avatarmedium"]	= session.Values["avatarmedium"].(string)
	}

	// Parse header.gohtml
	if t, err := template.ParseFiles("header.gohtml"); err == nil {
		parse["urlPath"]	= r.URL.Path
		parse["css"]		= []string{"/css/index.css"}
		parse["steamWallpaper"]	= steamWallpaper
		if err := t.Execute(w, parse); err != nil {
			log.Println(err)
		}
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Parse index.gohtml
	if t, err := template.ParseFiles(path); err == nil {
		// Append parameter 'variables' (map) to variable 'parse' (map)
		dv, sv := reflect.ValueOf(parse), reflect.ValueOf(variables)
		for _, k := range sv.MapKeys() {
			dv.SetMapIndex(k, sv.MapIndex(k))
		}

		if err := t.Execute(w, parse); err != nil {
			log.Println(err)
		}
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Parse footer.gohtml
	if t, err := template.ParseFiles("footer.gohtml"); err == nil {
		if err := t.Execute(w, parse); err != nil {
			log.Println(err)
		}
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}
}

// Page Handlers
func loginHandler(w http.ResponseWriter, r *http.Request) {
	// Check if url is different than wanted
	if r.URL.Path != "/login" {
		errorHandler(w, r, http.StatusNotFound)
		return
	}

	// Define a new open id
	opId := steam_auth.NewOpenId(r)

	// Handle each result
	switch opId.Mode() {
	case "":
		http.Redirect(w, r, opId.AuthUrl(), 301)
	case "cancel":
		w.Write([]byte("Authorization cancelled"))
	default:
		// Collect the steam id
		steamID, err := opId.ValidateAndGetId()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
			return
		}
		id, _ := strconv.ParseInt(steamID, 10, 64)

		// Collect user's info
		user, err := getUserDetails(id)
		if err != nil {
			w.Write([]byte("Unable to login"))
			return
		}

		// Connect to database
		db, err := sql.Open("mysql", "steaminviter:AriisAwesome9@tcp(45.32.189.171:3306)/steaminviter")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
			return
		}

		// Check connection status
		if err = db.Ping(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
			return
		}

		// Execute query
		rows, err := db.Query("SELECT steamid FROM users WHERE steamid='" + strconv.FormatInt(user.steamid, 10) + "'")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
			return
		}

		// Define query
		var query string
		if !rows.Next() {
			// ID doesn't exist
			query = `
				INSERT INTO
				users (
					steamid,
					personaname,
					avatar,
					lastip,
					timecreated
				)
				VALUES (
					'` + strconv.FormatInt(user.steamid, 10) + `',
					'` + user.personaname + `',
					'` + user.avatar + `',
					'` + strings.Split(r.RemoteAddr, ":")[0] + `',
					'` + strconv.FormatInt(makeTimestamp(), 10) + `'
				)
			`
		} else {
			// ID exists
			query = `
				UPDATE
					users
				SET
					personaname='` + user.personaname + `',
					avatar='` + user.avatar + `',
					lastip='` + strings.Split(r.RemoteAddr, ":")[0] + `'
				WHERE
					steamid='` + strconv.FormatInt(user.steamid, 10) + `'
			`
		}
		rows.Close()

		// Execute query
		rows, err = db.Query(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
			return
		}
		rows.Close()
		db.Close()

		// Connect to session
		session, err := store.Get(r, "user")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
			return
		}

		// Define session options
		session.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   86400 * 7,
		}

		// Set session values
		session.Values["steamid"]			= user.steamid
		session.Values["communityvisibilitystate"]	= user.communityvisibilitystate
		session.Values["profilestate"]			= user.profilestate
		session.Values["personaname"]			= user.personaname
		session.Values["lastlogoff"]			= user.lastlogoff
		session.Values["commentpermission"]		= user.commentpermission
		session.Values["profileurl"]			= user.profileurl
		session.Values["avatar"]			= user.avatar
		session.Values["avatarmedium"]			= user.avatarmedium
		session.Values["avatarfull"]			= user.avatarfull
		session.Values["personastate"]			= user.personastate

		// Save session values
		session.Save(r, w)

		// Redirect user
		http.Redirect(w, r, "/", 301)
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	// Check if url is different than wanted
	if r.URL.Path != "/logout" {
		errorHandler(w, r, http.StatusNotFound)
		return
	}

	// Connect to session
	session, err := store.Get(r, "user")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Delete session
	session.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   -1,
	}

	// Save session values
	session.Save(r, w)

	// Redirect user
	http.Redirect(w, r, "/", 301)
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
	// Connect to database
	db, err := sql.Open("mysql", "steaminviter:AriisAwesome9@tcp(45.32.189.171:3306)/steaminviter")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Check connection status
	if err = db.Ping(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Execute query
	rows, err := db.Query("SELECT personaname FROM users WHERE steamid='" + r.URL.Path[1:] + "'")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	var personaname string

	// Check if steam id exists
	if rows.Next() {
		err := rows.Scan(&personaname)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
			return
		}
		w.Write([]byte(personaname))
	} else {
		w.Write([]byte("No user found!"))
	}
	rows.Close()
	db.Close()
}

func ipnHandler(w http.ResponseWriter, r *http.Request) {
	// Payment has been received and IPN is verified.  This is where you
	// update your database to activate or process the order, or setup
	// the database with the user's order details, email an administrator,
	// etc. You can access a slew of information via the IPN data from r.Form

	// Check the paypal documentation for specifics on what information
	// is available in the IPN POST variables.  Basically, all the POST vars
	// which paypal sends, which we send back for validation.

	// For this tutorial, we'll just print out all the IPN data.

	err := r.ParseForm() // need this to get PayPal's HTTP POST of IPN data
	if err != nil {
		fmt.Println(err)
		return
	}

	if r.Method == "POST" {
		var postStr string = "https://www.sandbox.paypal.com/cgi-bin/webscr" + "&cmd=_notify-validate&"

		paymentInfo := make(map[string][]string)

		file, err := os.OpenFile("ipn", os.O_RDWR|os.O_CREATE, 0660)
		if err != nil {
			log.Println(err)
			return
		}

		var writeString string

		for k, v := range r.Form {
			//fmt.Println("key :", k)
			//fmt.Println("value :", strings.Join(v, ""))
			writeString += string(k[0]) + ": " + string(v[0]) + "\n"
			// NOTE : Store the IPN data k,v into a slice. It will be useful for database entry later.

			paymentInfo[k] = v
			postStr = postStr + k + "=" + url.QueryEscape(strings.Join(v, "")) + "&"
		}

		file.Write([]byte(writeString))

		// To verify the message from PayPal, we must send
		// back the contents in the exact order they were received and precede it with
		// the command _notify-validate

		// PayPal will then send one single-word message, either VERIFIED,
		// if the message is valid, or INVALID if the messages is not valid.

		// See more at
		// https://developer.paypal.com/webapps/developer/docs/classic/ipn/integration-guide/IPNIntro/

		// post data back to PayPal
		client := &http.Client{}
		req, err := http.NewRequest("POST", postStr, nil)

		if err != nil {
			fmt.Println(err)
			return
		}

		req.Header.Add("Content-Type: ", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)

		if err != nil {
			fmt.Println(err)
			return
		}

		// convert response to string
		respStr, _ := ioutil.ReadAll(resp.Body)

		//fmt.Println("Response String : ", string(respStr))

		verified, err := regexp.MatchString("VERIFIED", string(respStr))

		if err != nil {
			fmt.Println(err)
			return
		}

		if verified {
			fmt.Println("IPN verified")
			fmt.Println("TODO : Email receipt, increase credit, etc")
		} else {
			fmt.Println("IPN validation failed!")
			fmt.Println("Do not send the stuff out yet!")
		}

	}
}

// General Functions
func getUserDetails(steamid int64) (*steamUser, error) {
	resp, err := http.Get("http://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?" + url.Values{
		"key":		[]string{apikey},
		"steamids":	[]string{strconv.FormatInt(steamid, 10)},
	}.Encode())
	defer resp.Body.Close()
	if err != nil {
		return nil, errors.New("Unable to get player summaries.")
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("Unable to read player summaries.")
	}

	var decoded map[string]interface{}

	if err = json.Unmarshal(content, &decoded); err != nil {
		return nil, errors.New("Unable to unmarshal content.")
	}

	if len(decoded["response"].(map[string]interface{})["players"].(interface{}).([]interface{})) == 0 {
		return nil, errors.New("Empty response from player summaries.")
	}

	var user steamUser

	for key, val := range decoded["response"].(map[string]interface{})["players"].(interface{}).([]interface{})[0].(map[string]interface{}) {
		switch key {
		case "steamid":
			id, _ := strconv.ParseInt(val.(string), 10, 64)
			user.steamid			= id
		case "communityvisibilitystate":
			user.communityvisibilitystate	= int8(val.(float64))
		case "profilestate":
			user.profilestate		= int8(val.(float64))
		case "personaname":
			user.personaname		= val.(string)
		case "lastlogoff":
			user.lastlogoff			= int(val.(float64))
		case "commentpermission":
			user.commentpermission		= int8(val.(float64))
		case "profileurl":
			user.profileurl			= val.(string)
		case "avatar":
			user.avatar			= val.(string)
		case "avatarmedium":
			user.avatarmedium		= val.(string)
		case "avatarfull":
			user.avatarfull			= val.(string)
		case "personastate":
			user.personastate		= int8(val.(float64))
		}
	}

	return &user, nil
}

func sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/sendMessage" {
		errorHandler(w, r, http.StatusNotFound)
		return
	}

	r.ParseForm()

	name	:= r.FormValue("name")
	email	:= r.FormValue("email")
	message	:= r.FormValue("message")

	if err := mailTo("contact@steaminviter.com", "Contact - " + name, "<h3>" + name + " - " + email + ":</h3><p>" + message + "</p>"); err != nil {
		w.Write([]byte("ERR"))
		log.Println(err)
	} else {
		w.Write([]byte("OK"))
	}
}

func getSteamBackground() string {
	resp, err := http.Get("http://steamcommunity.com/")
	if err != nil {
		log.Println(err)
		return "http://cdn.akamai.steamstatic.com/steam/apps/201810/page_bg_generated_v6b.jpg?t=1447361219"
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return "http://cdn.akamai.steamstatic.com/steam/apps/201810/page_bg_generated_v6b.jpg?t=1447361219"
	}

	reg := regexp.MustCompile(`<div class="apphub_background" style="background-image: url\('(.+)'\);">`)
	if matches := reg.FindSubmatch(body); len(matches) > 1 {
		return string(matches[1])
	} else {
		log.Println("ojkewf")
		return "http://cdn.akamai.steamstatic.com/steam/apps/201810/page_bg_generated_v6b.jpg?t=1447361219"
	}
}

// Other Common Functions
func errorHandler(w http.ResponseWriter, r *http.Request, status int) {
	w.WriteHeader(status)
	if status == http.StatusNotFound {
		w.Write([]byte("Error 404 - Page not found."))
	}
}

func mailTo(to, subject, message string) error {
	m := gomail.NewMessage()
	m.SetAddressHeader("From", "ariseyhun9@gmail.com", "Steam Inviter Contact")
	m.SetAddressHeader("To", to, "Contact")
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", message)

	d := gomail.NewPlainDialer("smtp-pulse.com", 2525, "ariseyhun9@gmail.com", "aF5p6BCbgLm6Cfa")

	d.SSL = false
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	if err := d.DialAndSend(m); err != nil {
		return err
	}

	return nil
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}