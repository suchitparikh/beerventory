package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-martini/martini"
	_ "github.com/go-sql-driver/mysql"
)

type BeerList []Beer

func (b *BeerList) JSON() ([]byte, error) {
	return json.Marshal(b)
}

type Beer struct {
	Upc  string `json:"upc"`
	Type int    `json:"type"`
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}

func (b *Beer) JSON() ([]byte, error) {
	return json.Marshal(b)
}

type CheckoutEvent struct {
	Upc       string `json:"upc"`
	Timestamp int    `json:"timestamp,omitempty"`
	Location  int    `json:"location"`
}

type BeerType struct {
	Type string `json:"type"`
	Id   int    `json:"id"`
}

func main() {
	fmt.Println("Drink beer")
	host := "localhost"
	port := 3306

	mysqlAddr := fmt.Sprintf("root@tcp(%s:%d)/beerventory", host, port)
	db, err := sql.Open("mysql", mysqlAddr)
	if err != nil {
		log.Fatalf("Could not connect to mysql at %s:%s", host, port)
	}

	m := martini.Classic()
	m.Use(SetJsonContentType)

	m.Get("/type", func() (int, string) {
		res, err := db.Query("Select * from beer_type")
		if err != nil {
			log.Printf("Couldn't query for beer types")
			return 500, "No beer types here"
		}
		beerTypes := make([]BeerType, 0)
		for res.Next() {
			var curBeerType BeerType
			err := res.Scan(&curBeerType.Id, &curBeerType.Type)
			if err != nil {
				log.Print("No scan", err)
			}
			beerTypes = append(beerTypes, curBeerType)
		}
		beerTypeData, err := json.Marshal(beerTypes)
		if err != nil {
			return 500, "bad json go fish"
		}
		return 200, string(beerTypeData)
	})

	m.Post("/type", func(req *http.Request) (int, string) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Print("Couldn't read post body", err)
			return 400, "go fish"
		}
		log.Print("Post Type")
		var beerType BeerType
		err = json.Unmarshal(body, &beerType)
		if err != nil {
			log.Print("Couldn't unmarshal json", err)
			return 400, "go fish"
		}

		res, err := db.Exec(
			"insert into beer_type set type=?", beerType.Type,
		)
		if err != nil {
			log.Print("Couldn't save beer", err)
			return 400, "go fish"
		}
		lastId, _ := res.LastInsertId()

		return 200, fmt.Sprintf(`{"type":%s, "id":%d}`, beerType.Type, lastId)
	})

	m.Get("/beer", func() (int, string) {
		res, err := db.Query("Select * from beer")
		if err != nil {
			log.Printf("Couldn't query for beer")
			return 500, "No beer here"
		}

		// statusCode, beersJson := QueryMakerZero(res)
		beers := QueryMakerZero(res)
		beersJson, err := json.Marshal(beers)
		if err != nil {
			return 500, "Json marshalling error"
		}
		return 200, string(beersJson)
	})

	m.Get("/beer/:id", func(params martini.Params) (int, string) {
		log.Print("beer beer", params["id"])
		res, err := db.Query("Select * from beer where upc = ? limit 1", params["id"])
		if err != nil {
			log.Printf("Couldn't query for beer", err)
			return 500, "No beer here"
		}

		// statusCode, beersJson := QueryMakerZero(res)
		beers := QueryMakerZero(res)
		if len(beers) == 0 {
			return 404, "no beer"
		}

		beerJson, err := json.Marshal(beers[0])
		if err != nil {
			return 500, "Json marshalling error"
		}
		return 200, string(beerJson)
	})

	m.Post("/beer", func(req *http.Request) (int, string) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Print("Couldn't read post body", err)
			return 400, "go fish"
		}
		log.Print("Post beer")
		var beer Beer
		err = json.Unmarshal(body, &beer)
		if err != nil {
			log.Print("Couldn't unmarshal json", err)
			return 400, "go fish"
		}

		_, err = db.Exec(
			"insert into beer set upc=?, type_id=?, name=?, qty=?",
			beer.Upc, beer.Type, beer.Name, beer.Qty,
		)
		if err != nil {
			log.Print("Couldn't save beer", err)
			return 400, "go fish"
		}

		beerJson, _ := beer.JSON()
		return 200, string(beerJson)
	})

	m.Put("/beer/:upc", func(req *http.Request, params martini.Params) (int, string) {
		upc := params["upc"]
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Print("Couldn't read post body", err)
			return 400, "go fish"
		}
		log.Print("Post beer")
		var beer Beer
		err = json.Unmarshal(body, &beer)
		if err != nil {
			log.Print("Couldn't unmarshal json", err)
			return 400, "go fish"
		}
		beer.Upc = upc
		log.Print("Post beer", beer)
		_, err = db.Exec(
			"update beer set type_id=?, name=?, qty=? where upc=?",
			beer.Type, beer.Name, beer.Qty, upc,
		)
		if err != nil {
			log.Print("Couldn't save beer", err)
			return 400, "go fish"
		}

		beerJson, _ := beer.JSON()
		return 200, string(beerJson)
	})

	m.Delete("/beer/:upc", func(params martini.Params) (int, string) {
		res, err := db.Exec("delete from beer where upc=? limit 1", params["upc"])
		log.Print("deleting beer", params["upc"])
		if err != nil {
			log.Print("Couldn't delete beer", err)
			return 400, "go fish"
		}
		deleted, _ := res.RowsAffected()
		return 200, fmt.Sprintf(`{"deleted":%d}`, deleted)
	})

	m.Post("/checkout", func(req *http.Request) (int, string) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Print("Couldn't read body", err)
			return 400, "go fish"
		}

		var checkoutEvent CheckoutEvent
		err = json.Unmarshal(body, &checkoutEvent)
		checkoutEvent.Upc = strings.TrimSpace(checkoutEvent.Upc)
		if err != nil {
			log.Print("Couldn't unmarshal json", err)
			return 400, "go fish"
		}

		log.Print(checkoutEvent)
		_, err = db.Exec(
			"insert into event set upc=?, location_id=?, type='consumed'",
			checkoutEvent.Upc, checkoutEvent.Location,
		)
		if err != nil {
			log.Print("Can't save checkout", err)
			return 400, "go fish"
		}

		_, err = db.Exec(
			"update beer set qty=(qty-1) where upc=? limit 1",
			checkoutEvent.Upc,
		)
		if err != nil {
			log.Print("Can't update qty on checkout", err)
			return 400, "go fish"
		}

		go Notify(db, checkoutEvent.Upc)

		eventJson, err := json.Marshal(checkoutEvent)
		if err != nil {
			return 500, "Json marshalling error"
		}

		return 200, string(eventJson)
	})

	m.Run()
}

func Notify(db *sql.DB, upc string) {
	res, err := db.Query("select name, qty from beer where upc=? limit 1", upc)
	if err != nil {
		log.Print("error querying for quantity:", err)
		return
	}
	for res.Next() {
		var qty int
		var beer string
		err := res.Scan(&beer, &qty)
		if err != nil {
			log.Print("No scan on qty:", err)
			return
		}
		if qty == 0 {
            ran_out := fmt.Sprintf("We've run out of %s!", beer)

            // email
            addresses := make([]string, 2)
            addresses = append(addresses, "suchit@sendgrid.com", "richard.the@sendgrid.com")
            go sendEmail(addresses, ran_out, "Beerventory Update")

            // sms
			go sendSms(os.Getenv("ANAHEIM"), ran_out)
		}
		// else if qty < 3 {
		// 	go sendSms(os.Getenv("ANAHEIM"), beer)
		// }
	}
}

func SetJsonContentType(res http.ResponseWriter) {
	res.Header().Add("Content-Type", "application/json")
}

func QueryMakerZero(res *sql.Rows) []Beer {
	beers := make([]Beer, 0)
	for res.Next() {
		var curBeer Beer
		err := res.Scan(&curBeer.Upc, &curBeer.Name, &curBeer.Type, &curBeer.Qty)
		if err != nil {
			log.Print("No scan", err)
		}
		beers = append(beers, curBeer)
	}
	return beers
}
