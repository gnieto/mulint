package mulint

import (
	"fmt"
	"strings"
)

type FQN string

func FromCallInfo(pkg, fnName string) FQN {
	return FQN(strings.Trim(fmt.Sprintf("%s:%s", pkg, fnName), "*"))
}
