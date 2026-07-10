package services

import "time"

const appTimezoneName = "Asia/Manila"

func resolveAppLocation() (*time.Location, bool, error) {
	loc, err := time.LoadLocation(appTimezoneName)
	if err == nil {
		return loc, false, nil
	}

	return time.FixedZone("Asia/Manila_Fallback", 8*60*60), true, err
}

func parseAppDateRange(dateStr string) (time.Time, time.Time, *time.Location, bool, error, error) {
	loc, usedFallback, loadErr := resolveAppLocation()
	date, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return time.Time{}, time.Time{}, loc, usedFallback, loadErr, err
	}
	return date, date.Add(24 * time.Hour), loc, usedFallback, loadErr, nil
}

func utcNow() time.Time {
	return time.Now().UTC()
}
