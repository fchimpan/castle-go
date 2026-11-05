package castle_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	castle "github.com/fchimpan/castle-go"
	"github.com/fchimpan/castle-go/api"
)

func ExampleClient_Risk() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"type":"$login","policy":{"action":"challenge","name":null,"id":null,"revision_id":null},"signals":{},"device":{"fingerprint":null}}`)
	}))
	defer server.Close()
	client, err := castle.NewClient("synthetic-secret", castle.Config{
		BaseURL: server.URL,
	})
	if err != nil {
		panic(err)
	}
	var name, value api.Context_Headers_Item
	if err := name.FromContextHeaders0("User-Agent"); err != nil {
		panic(err)
	}
	if err := value.FromContextHeaders0("ExampleBrowser/1"); err != nil {
		panic(err)
	}
	var request api.Risk
	if err := request.FromLoginRiskRequest(api.LoginRiskRequest{
		Status:       api.LoginRiskRequestStatusSucceeded,
		RequestToken: new("synthetic-request-token"),
		Context: &api.Context{
			Ip:      "192.0.2.1",
			Headers: [][]api.Context_Headers_Item{{name, value}},
		},
		User: api.UserForMonitorAndRisk{
			Id: "user-123",
		},
	}); err != nil {
		panic(err)
	}
	result, err := client.Risk(context.Background(), request)
	if err != nil {
		panic(err)
	}
	login, err := result.JSON201.AsLoginResponse()
	if err != nil {
		panic(err)
	}
	action, err := login.Policy.Action.Get()
	if err != nil {
		panic(err)
	}
	fmt.Println(action)
	fmt.Println(login.Scores == nil)
	// Output:
	// challenge
	// true
}
