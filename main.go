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
)

const apikey string	= "2B2A0C37AC20B5DC2234E579A2ABB11C"
var store = sessions.NewCookieStore([]byte("AriisAwesome9"))

type steamUser struct {
	steamid				int64
	communityvisibilitystate	int8		// 1 = Private, 3 = Public
	profilestate			int8		// 0 = Profile not setup, 1 = Profile setup
	personaname			string		// Steam Name
	lastlogoff			int
	commentpermission		int8
	profileurl			string
	avatar				string		// 32x32px avatar
	avatarmedium			string		// 64x64px avatar
	avatarfull			string		// 184x184px avatar
	personastate			int8		// 0 = Offline, 1 = Online, 2 = Busy, 3 = Away, 4 = Snooze, 5 = Looking to Trade, 6 = Looking to Play
}

func main() {
	cssHandler := http.FileServer(http.Dir("./css/"))
	imagesHandler := http.FileServer(http.Dir("./images/"))

	http.Handle("/css/", http.StripPrefix("/css/", cssHandler))
	http.Handle("/images/", http.StripPrefix("/images/", imagesHandler))

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/purchase", purchaseHandler)

	log.Println("Listening to :8080")
	http.ListenAndServe(":8080", nil)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		errorHandler(w, r, http.StatusNotFound)
		return
	}

	p := make(map[string]string)
	if t, err := template.ParseFiles("index.gohtml"); err == nil {
		session, err := store.Get(r, "user")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if !session.IsNew {
			p["steamid"]		= strconv.FormatInt(session.Values["steamid"].(int64), 10)
			p["personaname"]	= session.Values["personaname"].(string)
			p["avatarmedium"]	= session.Values["avatarmedium"].(string)
		}

		t.Execute(w, p)
	} else {
		log.Print(err)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/login" {
		errorHandler(w, r, http.StatusNotFound)
		return
	}

	opId := steam_auth.NewOpenId(r)
	switch opId.Mode() {
	case "":
		http.Redirect(w, r, opId.AuthUrl(), 301)
	case "cancel":
		w.Write([]byte("Authorization cancelled"))
	default:
		steamId, err := opId.ValidateAndGetId()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		id, _ := strconv.ParseInt(steamId, 10, 64)

		user, err := getUserDetails(id)
		if err != nil {
			w.Write([]byte("Unable to login"))
			return
		}

		session, err := store.Get(r, "user")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

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

		session.Save(r, w)

		http.Redirect(w, r, "/", 301)
	}
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