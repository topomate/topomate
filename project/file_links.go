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
		utils.Fatalln("internalFromFile:", err)
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
			utils.Fatalf("internalFromFile: error parsing speed at line %d, %v\n", current, err)
		}
		l.First.Interface.SetSpeedAndCost(speed)
		l.Second.Interface.SetSpeedAndCost(speed)

		if len(fields) > 3 {
			cost, err := strconv.Atoi(fields[3])
			if err != nil {
				utils.Fatalf("internalFromFile: error parsing IGP cost at line %d, %v\n", current, err)
			}
			l.First.Interface.Cost = cost
			l.Second.Interface.Cost = cost
		}
		if len(fields) > 4 {
			cost, err := strconv.Atoi(fields[4])
			if err != nil {
				utils.Fatalf("internalFromFile: error parsing IGP cost at line %d, %v\n", current, err)
			}
			l.Second.Interface.Cost = cost
		}

		l.First.Interface.Description = fmt.Sprintf("linked to %s", l.Second.Router.Hostname)
		l.Second.Interface.Description = fmt.Sprintf("linked to %s", l.First.Router.Hostname)
		res = append(res, l)
	}
	if err := scanner.Err(); err != nil {
		utils.Fatalln("internalFromFile:", err)
	}
	return res
}

func (p *Project) externalFromFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		utils.Fatalln("internalFromFile:", err)
	}
	scanner := bufio.NewScanner(f)
	current := 0
	for scanner.Scan() {
		current++
		fields := strings.Fields(scanner.Text())

		from := strings.SplitN(fields[0], ".", 2)
		to := strings.SplitN(fields[1], ".", 2)

		fromASN, err := strconv.Atoi(from[0])
		if err != nil {
			utils.Fatalf("externalFromFile: error parsing ASN at line %d (%s)\n", current, from[0])
		}
		fromRID, err := strconv.Atoi(from[1])
		if err != nil {
			utils.Fatalf("externalFromFile: error parsing router number at line %d (%s)\n", current, from[1])
		}
		toASN, err := strconv.Atoi(to[0])
		if err != nil {
			utils.Fatalf("externalFromFile: error parsing ASN at line %d (%s)\n", current, to[0])
		}
		toRID, err := strconv.Atoi(to[1])
		if err != nil {
			utils.Fatalf("externalFromFile: error parsing router number at line %d (%s)\n", current, to[1])
		}

		l := &ExternalLink{
			From: NewExtLinkItem(
				fromASN,
				p.AS[fromASN].Routers[fromRID-1],
			),
			To: NewExtLinkItem(
				toASN,
				p.AS[toASN].Routers[toRID-1],
			),
		}

		speed, err := strconv.Atoi(fields[3])
		if err != nil {
			utils.Fatalf("externalFromFile: error parsing speed at line %d, %v\n", current, err)
		}

		l.From.Interface.SetSpeedAndCost(speed)
		l.To.Interface.SetSpeedAndCost(speed)

		switch strings.ToLower(fields[2]) {
		case "p2c":
			l.From.Relation = Provider
			l.To.Relation = Customer
			break
		case "c2p":
			l.From.Relation = Customer
			l.To.Relation = Provider
			break
		case "p2p":
			l.From.Relation = Peer
			l.To.Relation = Peer
			break
		default:
			break
		}
		l.setupExternal(&p.AS[fromASN].Network.NextAvailable)
		p.Ext = append(p.Ext, l)
	}
}
