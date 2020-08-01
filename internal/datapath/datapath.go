package datapath

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/rahveiz/topomate/internal/ovsdocker"

	"github.com/rahveiz/topomate/utils"
	"github.com/weaveworks/go-odp/odp"
)

func lookupDatapath(dpif *odp.Dpif, name string) (*odp.DatapathHandle, error) {
	dph, err := dpif.LookupDatapath(name)
	if err == nil {
		return &dph, nil
	}

	if !odp.IsNoSuchDatapathError(err) {
		return nil, fmt.Errorf("%s", err)
	}

	// If the name is a number, try to use it as an id
	ifindex, err := strconv.ParseUint(name, 10, 32)
	if err == nil {
		dp, err := dpif.LookupDatapathByID(odp.DatapathID(ifindex))
		if err == nil {
			return &dp.Handle, nil
		}

		if !odp.IsNoSuchDatapathError(err) {
			return nil, fmt.Errorf("%s", err)
		}
	}

	return nil, fmt.Errorf("Cannot find datapath \"%s\"", name)
}

func CreateDP(name string) error {
	dpif, err := odp.NewDpif()
	if err != nil {
		return err
	}
	defer dpif.Close()

	_, err = dpif.CreateDatapath(name)
	if err != nil {
		fmt.Println("oof")
		if odp.IsDatapathNameAlreadyExistsError(err) {
			return fmt.Errorf("Network device named %s already exists", name)
		} else {
			return fmt.Errorf("%s", err)
		}
	}
	return nil
}

func AddVPort(dpname, dev string) error {
	dpif, err := odp.NewDpif()
	if err != nil {
		return fmt.Errorf("%s", err)
	}
	defer dpif.Close()

	dp, err := lookupDatapath(dpif, dpname)
	if err != nil {
		utils.Fatalln(err)
	}

	_, err = dp.CreateVport(odp.NewNetdevVportSpec(dev))
	if err != nil {
		return fmt.Errorf("%s", err)
	}
	return nil
}

const ETH_ALEN = odp.ETH_ALEN

func parseMAC(s string) (mac [6]byte, err error) {
	hwa, err := net.ParseMAC(s)
	if err != nil {
		return
	}

	if len(hwa) != 6 {
		err = fmt.Errorf("invalid MAC address: %s", s)
		return
	}

	copy(mac[:], hwa)
	return
}

func handleEthernetAddrOption(opt string) (key [ETH_ALEN]byte, mask [ETH_ALEN]byte, err error) {
	if opt != "" {
		var k, m string
		i := strings.Index(opt, "&")
		if i > 0 {
			k = opt[:i]
			m = opt[i+1:]
		} else {
			k = opt
			m = "ff:ff:ff:ff:ff:ff"
		}

		key, err = parseMAC(k)
		if err != nil {
			return
		}

		mask, err = parseMAC(m)
	}

	return
}

func handleEthernetFlowKeyOptions(flow odp.FlowSpec, src string, dst string) error {
	var err error
	takeErr := func(key [ETH_ALEN]byte, mask [ETH_ALEN]byte,
		e error) ([ETH_ALEN]byte, [ETH_ALEN]byte) {
		err = e
		return key, mask
	}

	fk := odp.NewEthernetFlowKey()

	fk.SetMaskedEthSrc(takeErr(handleEthernetAddrOption(src)))
	fk.SetMaskedEthDst(takeErr(handleEthernetAddrOption(dst)))

	if err != nil {
		return err
	}

	flow.AddKey(fk)
	return nil
}

func flagsToFlowSpec(dpif *odp.Dpif, dpName, inPort, output string) (dp odp.DatapathHandle, flow odp.FlowSpec, ok bool) {
	flow = odp.NewFlowSpec()

	dpp, err := lookupDatapath(dpif, dpName)
	if err != nil {
		return
	}

	if inPort != "" {
		vport, err := dpp.LookupVportByName(inPort)
		if err != nil {
			fmt.Printf("%s", err)
			return
		}
		flow.AddKey(odp.NewInPortFlowKey(vport.ID))
	}

	// The ethernet flow key is mandatory
	err = handleEthernetFlowKeyOptions(flow, "", "")
	if err != nil {
		fmt.Printf("%s", err)
		return
	}

	if output != "" {
		for _, vpname := range strings.Split(output, ",") {
			vport, err := dpp.LookupVportByName(vpname)
			if err != nil {
				fmt.Printf("%s", err)
				return
			}
			flow.AddAction(odp.NewOutputAction(vport.ID))
		}
	}

	return *dpp, flow, true
}

func AddFlow(dpName, inIf, outIf string) error {
	dpif, err := odp.NewDpif()
	if err != nil {
		return fmt.Errorf("%s", err)
	}
	defer dpif.Close()

	dp, flow, ok := flagsToFlowSpec(dpif, dpName, inIf, outIf)
	if !ok {
		return fmt.Errorf("Failed to parse flow (dp: %s, in: %s, out: %s)", dpName, inIf, outIf)
	}

	err = dp.CreateFlow(flow)
	if err != nil {
		return fmt.Errorf("%s", err)
	}

	return nil
}

func DeleteDatapath(dpName string) error {
	dpif, err := odp.NewDpif()
	if err != nil {
		return err
	}
	defer dpif.Close()

	dp, err := lookupDatapath(dpif, dpName)
	if dp == nil {
		return err
	}

	err = dp.Delete()
	if err != nil {
		return err
	}

	return nil
}

func DeleteVPort(dev string) error {
	dpif, err := odp.NewDpif()
	if err != nil {
		return fmt.Errorf("%s", err)
	}
	defer dpif.Close()

	dp, vport, err := dpif.LookupVportByName(dev)
	if err != nil {
		if odp.IsNoSuchVportError(err) {
			return fmt.Errorf("Cannot find port \"%s\"", dev)
		}

		return fmt.Errorf("%s", err)
	}

	err = dp.DeleteVport(vport.ID)
	if err != nil {
		return fmt.Errorf("%s", err)
	}

	return nil
}

func DeleteFlow(dpName, inIf, outIf string) error {
	dpif, err := odp.NewDpif()
	if err != nil {
		return fmt.Errorf("%s", err)
	}
	defer dpif.Close()

	dp, flow, ok := flagsToFlowSpec(dpif, dpName, inIf, outIf)
	if !ok {
		return fmt.Errorf("Failed to delete flow")
	}

	err = dp.DeleteFlow(flow.FlowKeys)
	if err != nil {
		if odp.IsNoSuchFlowError(err) {
			return fmt.Errorf("No such flow (dp: %s, in: %s, out: %s)", dpName, inIf, outIf)
		} else {
			return fmt.Errorf("%s", err)
		}
	}

	return nil
}

func ProcessLinks(dpName string, linksConfig ovsdocker.OVSBulk) {
	if err := CreateDP(dpName); err != nil {
		utils.Fatalln(err)
	}

	processed := make(map[string]bool, 256)
	var wg sync.WaitGroup
	for _, ifaces := range linksConfig {
		for _, iface := range ifaces {
			if _, ok := processed[iface.HostIface]; ok {
				continue
			}
			wg.Add(1)
			processed[iface.HostIface] = true
			processed[iface.NbrIface] = true
			go func(w *sync.WaitGroup, a, b string) {
				AddVPort(dpName, a)
				AddVPort(dpName, b)
				AddFlow(dpName, a, b)
				AddFlow(dpName, b, a)
				w.Done()
			}(&wg, iface.HostIface, iface.HostIface)
		}
	}

	wg.Wait()
}

func ProcessLinksSeq(dpName string, linksConfig ovsdocker.OVSBulk) {
	if err := CreateDP(dpName); err != nil {
		utils.Fatalln(err)
	}

	processed := make(map[string]bool, 256)
	for _, ifaces := range linksConfig {
		for _, iface := range ifaces {
			// Already processed
			if _, ok := processed[iface.HostIface]; ok {
				continue
			}
			if err := AddVPort(dpName, iface.HostIface); err != nil {
				utils.Fatalln(err)
			}
			if err := AddVPort(dpName, iface.NbrIface); err != nil {
				utils.Fatalln(err)
			}
			if err := AddFlow(dpName, iface.HostIface, iface.NbrIface); err != nil {
				utils.Fatalln(err)
			}
			if err := AddFlow(dpName, iface.NbrIface, iface.HostIface); err != nil {
				utils.Fatalln(err)
			}
			processed[iface.HostIface] = true
			processed[iface.NbrIface] = true
		}
	}
}
