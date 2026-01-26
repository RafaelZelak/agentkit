package funcs

import (
	"fmt"
	"time"

	"github.com/RafaelZelak/agentkit/sdk"
)

func init() {
	sdk.RegisterGoFunction("time.now", Now)
	sdk.RegisterGoFunction("time.greeting", Greeting)
}

func Now() string {
	hour := time.Now().Hour()
	
	if hour >= 6 && hour < 12 {
		return "Bom Dia"
	} else if hour >= 12 && hour < 18 {
		return "Boa Tarde"
	} else {
		return "Boa Noite"
	}
}

func Greeting(name string) string {
	greeting := Now()
	return fmt.Sprintf("%s, %s", greeting, name)
}
