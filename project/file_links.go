package project

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rahveiz/topomate/utils"
)

func (a *AutonomousSystem) internalFromFile(path string) []Link {
	f, err := os.Open(path)
	if err != nil {
		utils.Fatalln("manualFromFile:", err)
	}
	res := make([]Link, 0, 256)
	scanner := bufio.NewScanner(f)
	current := 0
	for scanner.Scan() {
		current++
		fields := strings.Fields(scanner.Text())
		l := Link{
			First:  NewLinkItem(a.getRouter(fields[0])),
			Second: NewLinkItem(a.getRouter(fields[1])),
		}
		speed, err := strconv.Atoi(fields[2])
		if err != nil {
			utils.Fatalf("manualFromFile: error parsing speed at line %d, %v\n", current, err)
		}
		l.First.Interface.SetSpeedAndCost(speed)
		l.Second.Interface.SetSpeedAndCost(speed)

		if len(fields) > 3 {
			cost, err := strconv.Atoi(fields[3])
			if err != nil {
				utils.Fatalf("manualFromFile: error parsing IGP cost at line %d, %v\n", current, err)
			}
			l.First.Interface.Cost = cost
			l.Second.Interface.Cost = cost
		}
		if len(fields) > 4 {
			cost, err := strconv.Atoi(fields[4])
			if err != nil {
				utils.Fatalf("manualFromFile: error parsing IGP cost at line %d, %v\n", current, err)
			}
			l.Second.Interface.Cost = cost
		}

		l.First.Interface.Description = fmt.Sprintf("linked to %s", l.Second.Router.Hostname)
		l.Second.Interface.Description = fmt.Sprintf("linked to %s", l.First.Router.Hostname)
		res = append(res, l)
	}
	if err := scanner.Err(); err != nil {
		utils.Fatalln("manualFromFile:", err)
	}
	return res
}
