package main

import ()

type BindDnsInstance struct {
	dns_backend_point
}

func buildZone(name string, zoneData *ZoneData) Zone {

	records := make([]Record, 0, 100)

	if zoneData != nil {
		for name, entry := range zoneData.Entries {
			for t, contents := range entry.Types {
				for _, content := range contents {
					record := Record{
						Name:    name,
						Content: content.Content,
						Type:    t,
					}
					records = append(records, record)
				}
			}
		}
	}

	zone := Zone{
		Name:    name,
		Records: records,
	}

	return zone
}

// List function
func (di *BindDnsInstance) GetAllZones(zones *ZoneTracker) ([]Zone, *backendError) {
	answer := make([]Zone, 0, 10)
	for k, v := range zones.Zones {
		answer = append(answer, buildZone(k, v))
	}

	return answer, nil
}

// Get function
func (di *BindDnsInstance) GetZone(zones *ZoneTracker, id string) (Zone, *backendError) {
	zdata := zones.Zones[id]
	if zdata == nil {
		return Zone{}, &backendError{"Not Found", 404}
	}

	return buildZone(id, zdata), nil
}

// Patch function
func (di *BindDnsInstance) PatchZone(zones *ZoneTracker, zoneName string, rec Record) (Zone, *backendError) {

	// Mutex
	// Rebuild zone files.
	// Update index
	// Restart bind
	// unMutex

	return buildZone(zoneName, zones.Zones[zoneName]), nil
}
