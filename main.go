package main

import (
	"log"
	"net/http"
	"html/template"
	"github.com/solovev/steam_go"
	"net/url"
	"errors"
	"io/ioutil"
	"encoding/json"
	"strconv"
	"github.com/gorilla/sessions"
	"gopkg.in/gomail.v2"
	"crypto/tls"
	"database/sql"
	"github.com/go-sql-driver/mysql"
	"time"
	"strings"
	"regexp"
)

const apikey	string	= "2B2A0C37AC20B5DC2234E579A2ABB11C"
var store		= sessions.NewCookieStore([]byte("secured-cookies"))
var query 	string

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
	cssHandler := http.FileServer(http.Dir("./css/"))
	jsHandler := http.FileServer(http.Dir("./js/"))
	imagesHandler := http.FileServer(http.Dir("./images/"))

	http.Handle("/css/", http.StripPrefix("/css/", cssHandler))
	http.Handle("/js/", http.StripPrefix("/js/", jsHandler))
	http.Handle("/images/", http.StripPrefix("/images/", imagesHandler))

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/purchase", purchaseHandler)
	http.HandleFunc("/contact", contactHandler)
	http.HandleFunc("/sendMessage", sendMessageHandler)

	log.Println("Listening to :8080")
	http.ListenAndServe(":8080", nil)

	_ = mysql.Config{}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	// Check if url is a profile
	if match, _ := regexp.MatchString(`^\d+$`, r.URL.Path[1:]); len(r.URL.Path[1:]) == 17 && match {
		profileHandler(w, r)
		return
	}
	
	// Check if url is different than wanted
	if r.URL.Path != "/" {
		errorHandler(w, r, http.StatusNotFound)
		return
	}

	// Define template variables
	parse			:= make(map[string]interface{})
	parse["steamid"]	= 0

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
		if err := t.Execute(w, parse); err != nil {
			log.Println(err)
		}
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Parse index.gohtml
	if t, err := template.ParseFiles("index.gohtml"); err == nil {
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

func contactHandler(w http.ResponseWriter, r *http.Request) {
	// Check if url is different than wanted
	if r.URL.Path != "/contact" {
		errorHandler(w, r, http.StatusNotFound)
		return
	}

	// Define template variables
	parse			:= make(map[string]interface{})
	parse["steamid"]	= 0

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
		if err := t.Execute(w, parse); err != nil {
			log.Println(err)
		}
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Parse contact.gohtml
	if t, err := template.ParseFiles("contact.gohtml"); err == nil {
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
		parse["js"] = []string{"/js/contact.js"}
		if err := t.Execute(w, parse); err != nil {
			log.Println(err)
		}
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}
}

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

func purchaseHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/purchase" {
		errorHandler(w, r, http.StatusNotFound)
		return
	}

	plan := r.URL.Query().Get("plan")

	p := make(map[string]string)
	if t, err := template.ParseFiles("purchase.gohtml"); err == nil {
		session, err := store.Get(r, "user")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
			return
		}

		if session.IsNew {
			http.Redirect(w, r, "/", 301)
			return
		}

		if plan != "toddler" && plan != "man" && plan != "god" {
			w.Write([]byte("Invalid Plan"))
			return
		}

		p["plan"] = plan

		t.Execute(w, p)
	} else {
		log.Print(err)
	}
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

func errorHandler(w http.ResponseWriter, r *http.Request, status int) {
	w.WriteHeader(status)
	if status == http.StatusNotFound {
		w.Write([]byte("Error 404 - Page not found."))
	}
}

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

	log.SetPrefix("HEY!!!")

	r.ParseForm()

	name	:= r.FormValue("name")
	email	:= r.FormValue("email")
	message	:= r.FormValue("message")

	if err := mailTo("ariseyhun9@gmail.com", "Contact - " + name, "<h3>" + name + " - " + email + ":</h3><p>" + message + "</p>"); err != nil {
		w.Write([]byte("ERR"))
		log.Fatal(err)
	} else {
		w.Write([]byte("OK"))
	}
}

func mailTo(to, subject, message string) error {
	m := gomail.NewMessage()
	m.SetAddressHeader("From", "ariseyhun@live.com.au", "Steam Inviter Contact")
	m.SetAddressHeader("To", to, "Contact")
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", message)

	d := gomail.NewPlainDialer("smtp-pulse.com", 2525, "ariseyhun@live.com.au", "a4Kg2XQ6Xp")

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