package main

import (
	"encoding/json"
	"fmt"

	"github.com/errordeveloper/kubegen/appmaker/assets/sockshop"
)

func main() {
	app := sockshop.MakeSockShop()
	generated, err := app.MarshalToCombinedJSON()
	if err != nil {
		panic(err)
	}

	manifest, err := json.Marshal(app)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s\n%s\n", string(generated), string(manifest))
}
