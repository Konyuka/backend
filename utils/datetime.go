package utils

import "time"

// DHM - returns Date Hour Minute
func DHM(date time.Time) string {

	layout := "2006-01-02 15:04:05 -0700 MST"

	d, err := time.Parse(layout, date.String())

	if err != nil {
		return time.Time{}.Format("2006-01-02 15:04:00")
	}

	return d.Format("2006-01-02 15:04:00")
}
