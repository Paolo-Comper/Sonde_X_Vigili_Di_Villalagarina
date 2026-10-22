// Package store salva su SQLite le letture normalizzate.
//
// Lo schema e' volutamente "largo" (una colonna nullable per grandezza fisica)
// cosi' che una sola tabella contenga sia le sonde idrometriche sia le stazioni
// MIMOSA, esportabile direttamente in CSV o interrogabile con SQL.
package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"

	"sonde/internal/telemetry"
)

// Store incapsula la connessione SQLite.
type Store struct {
	db *sql.DB
}

// Device e' una sonda vista almeno una volta.
type Device struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Group     string   `json:"group"`
	Kind      string   `json:"kind"`
	Topic     string   `json:"topic"`
	Lat       *float64 `json:"lat"`
	Lon       *float64 `json:"lon"`
	FirstSeen int64    `json:"first_seen"`
	LastSeen  int64    `json:"last_seen"`
	Readings  int64    `json:"readings"`
}

// Measurement e' una riga della tabella measurements.
type Measurement struct {
	ID          int64    `json:"id"`
	DeviceID    string   `json:"device_id"`
	DeviceLabel string   `json:"device_label"`
	Topic       string   `json:"topic"`
	Kind        string   `json:"kind"`
	TS          int64    `json:"ts"`
	WaterLevel  *float64 `json:"water_level_cm"`
	Ultrasonic  *float64 `json:"ultrasonic_cm"`
	Tof         *float64 `json:"tof_cm"`
	Value       *float64 `json:"value"`

	PM1     *float64 `json:"pm1"`
	PM25    *float64 `json:"pm25"`
	PM10    *float64 `json:"pm10"`
	TempPMS *float64 `json:"temp_pms"`
	HumPMS  *float64 `json:"hum_pms"`

	TempDHT *float64 `json:"temp_dht22"`
	HumDHT  *float64 `json:"hum_dht22"`

	TempBME  *float64 `json:"temp_bme280"`
	HumBME   *float64 `json:"hum_bme280"`
	PressBME *float64 `json:"press_bme280"`

	TempVirt  *float64 `json:"temp_virtual"`
	HumVirt   *float64 `json:"hum_virtual"`
	PressVirt *float64 `json:"press_virtual"`

	Lat  *float64 `json:"lat"`
	Lon  *float64 `json:"lon"`
	RSSI *int     `json:"rssi"`
	Net  *string  `json:"net"`

	Exported bool `json:"exported"`
}

// Stats e' il riepilogo mostrato nell'intestazione della dashboard.
type Stats struct {
	Devices  int64  `json:"devices"`
	Readings int64  `json:"readings"`
	Exported int64  `json:"exported"`
	Pending  int64  `json:"pending"`
	LastTS   *int64 `json:"last_ts"`
}

// SchemaColumn descrive una colonna della tabella measurements.
type SchemaColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

const measurementColumns = `id, device_id, device_label, topic, kind, ts,
 water_level_cm, ultrasonic_cm, tof_cm, value,
 pm1, pm25, pm10, temp_pms, hum_pms,
 temp_dht22, hum_dht22, temp_bme280, hum_bme280, press_bme280,
 temp_virtual, hum_virtual, press_virtual,
 lat, lon, rssi, net, exported`

// Open apre (creandolo se serve) il database SQLite al percorso indicato.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close chiude il database.
func (s *Store) Close() error { return s.db.Close() }

// DB espone l'handle sottostante (usato dai test).
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS devices (
			id         TEXT PRIMARY KEY,
			label      TEXT NOT NULL DEFAULT '',
			group_name TEXT NOT NULL DEFAULT '',
			kind       TEXT NOT NULL DEFAULT '',
			topic      TEXT NOT NULL DEFAULT '',
			lat        REAL,
			lon        REAL,
			first_seen INTEGER NOT NULL,
			last_seen  INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS measurements (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id      TEXT NOT NULL,
			device_label   TEXT NOT NULL DEFAULT '',
			topic          TEXT NOT NULL DEFAULT '',
			kind           TEXT NOT NULL DEFAULT '',
			ts             INTEGER NOT NULL,

			water_level_cm REAL,
			ultrasonic_cm  REAL,
			tof_cm         REAL,
			value          REAL,

			pm1      REAL, pm25 REAL, pm10 REAL,
			temp_pms REAL, hum_pms REAL,

			temp_dht22 REAL, hum_dht22 REAL,

			temp_bme280 REAL, hum_bme280 REAL, press_bme280 REAL,

			temp_virtual REAL, hum_virtual REAL, press_virtual REAL,

			lat REAL, lon REAL,
			rssi INTEGER, net TEXT,

			exported    INTEGER NOT NULL DEFAULT 0,
			exported_at INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_meas_device_ts ON measurements(device_id, ts)`,
		`CREATE INDEX IF NOT EXISTS idx_meas_ts ON measurements(ts)`,
		`CREATE INDEX IF NOT EXISTS idx_meas_exported ON measurements(exported)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// Insert salva una lettura e aggiorna l'anagrafica dispositivi in transazione.
func (s *Store) Insert(r telemetry.Reading) error {
	ts := r.TS.Unix()
	if r.TS.IsZero() {
		ts = time.Now().Unix()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO devices (id, label, group_name, kind, topic, lat, lon, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			label      = CASE WHEN excluded.label <> excluded.id THEN excluded.label ELSE devices.label END,
			group_name = excluded.group_name,
			kind       = excluded.kind,
			topic      = excluded.topic,
			lat        = COALESCE(excluded.lat, devices.lat),
			lon        = COALESCE(excluded.lon, devices.lon),
			last_seen  = excluded.last_seen`,
		r.DeviceID, r.Label, r.Group, string(r.Kind), r.Topic,
		nf(r.Lat), nf(r.Lon), ts, ts,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO measurements (
			device_id, device_label, topic, kind, ts,
			water_level_cm, ultrasonic_cm, tof_cm, value,
			pm1, pm25, pm10, temp_pms, hum_pms,
			temp_dht22, hum_dht22,
			temp_bme280, hum_bme280, press_bme280,
			temp_virtual, hum_virtual, press_virtual,
			lat, lon, rssi, net
		) VALUES (?,?,?,?,?, ?,?,?,?, ?,?,?,?,?, ?,?, ?,?,?, ?,?,?, ?,?,?,?)`,
		r.DeviceID, r.Label, r.Topic, string(r.Kind), ts,
		nf(r.WaterLevelCM), nf(r.UltrasonicCM), nf(r.TofCM), nf(r.Value),
		nf(r.PM1), nf(r.PM25), nf(r.PM10), nf(r.TempPMS), nf(r.HumPMS),
		nf(r.TempDHT), nf(r.HumDHT),
		nf(r.TempBME), nf(r.HumBME), nf(r.PressBME),
		nf(r.TempVirt), nf(r.HumVirt), nf(r.PressVirt),
		nf(r.Lat), nf(r.Lon), ni(r.RSSI), ns(r.Net),
	); err != nil {
		return err
	}

	return tx.Commit()
}

// ListDevices restituisce tutti i dispositivi noti con il numero di misure.
func (s *Store) ListDevices() ([]Device, error) {
	rows, err := s.db.Query(`
		SELECT d.id, d.label, d.group_name, d.kind, d.topic, d.lat, d.lon,
		       d.first_seen, d.last_seen,
		       (SELECT COUNT(*) FROM measurements m WHERE m.device_id = d.id)
		FROM devices d
		ORDER BY d.label COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Device
	for rows.Next() {
		var d Device
		var lat, lon sql.NullFloat64
		if err := rows.Scan(&d.ID, &d.Label, &d.Group, &d.Kind, &d.Topic,
			&lat, &lon, &d.FirstSeen, &d.LastSeen, &d.Readings); err != nil {
			return nil, err
		}
		d.Lat, d.Lon = pf(lat), pf(lon)
		out = append(out, d)
	}
	return out, rows.Err()
}

// Latest restituisce l'ultima misura di ogni dispositivo.
func (s *Store) Latest() ([]Measurement, error) {
	rows, err := s.db.Query(`
		SELECT ` + measurementColumns + `
		FROM measurements m
		WHERE m.id = (
			SELECT id FROM measurements
			WHERE device_id = m.device_id
			ORDER BY ts DESC, id DESC LIMIT 1
		)
		ORDER BY m.device_label COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	return scanAll(rows)
}

// Readings restituisce fino a limit misure, dalla piu' vecchia. Se deviceID
// non e' vuoto filtra quel solo dispositivo.
func (s *Store) Readings(deviceID string, limit int) ([]Measurement, error) {
	if limit <= 0 || limit > 100000 {
		limit = 2000
	}
	var (
		rows *sql.Rows
		err  error
	)
	if deviceID == "" {
		rows, err = s.db.Query(`
			SELECT `+measurementColumns+` FROM (
				SELECT * FROM measurements ORDER BY ts DESC, id DESC LIMIT ?
			) sub ORDER BY ts ASC, id ASC`, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT `+measurementColumns+` FROM (
				SELECT * FROM measurements WHERE device_id = ?
				ORDER BY ts DESC, id DESC LIMIT ?
			) sub ORDER BY ts ASC, id ASC`, deviceID, limit)
	}
	if err != nil {
		return nil, err
	}
	return scanAll(rows)
}

// Rows e' il sottoinsieme di *sql.Rows usato dall'exporter.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Columns() ([]string, error)
	ColumnTypes() ([]*sql.ColumnType, error)
	Err() error
	Close() error
}

// QueryRows esegue una query read-only validata e restituisce le righe grezze.
// La query gira in una transazione annullata alla Close: anche un'istruzione
// che scrivesse per errore non verrebbe salvata.
func (s *Store) QueryRows(query string) (Rows, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(query)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return &txRows{Rows: rows, tx: tx}, nil
}

// txRows chiude la transazione insieme alle righe.
type txRows struct {
	*sql.Rows
	tx *sql.Tx
}

func (r *txRows) Close() error {
	err := r.Rows.Close()
	if rerr := r.tx.Rollback(); err == nil {
		err = rerr
	}
	return err
}

// MarkExported segna come esportate le misure con gli id indicati.
func (s *Store) MarkExported(ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var total int64
	now := time.Now().Unix()
	for start := 0; start < len(ids); start += 500 {
		end := start + 500
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		ph := strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")
		args := make([]any, 0, len(chunk)+1)
		args = append(args, now)
		for _, id := range chunk {
			args = append(args, id)
		}
		res, err := s.db.Exec(
			`UPDATE measurements SET exported = 1, exported_at = ? WHERE id IN (`+ph+`)`,
			args...)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}

// ClearExported elimina tutte le misure gia' segnate come esportate.
func (s *Store) ClearExported() (int64, error) {
	res, err := s.db.Exec(`DELETE FROM measurements WHERE exported = 1`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Stats restituisce i contatori aggregati.
func (s *Store) Stats() (Stats, error) {
	var st Stats
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM devices`).Scan(&st.Devices); err != nil {
		return st, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM measurements`).Scan(&st.Readings); err != nil {
		return st, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM measurements WHERE exported = 1`).Scan(&st.Exported); err != nil {
		return st, err
	}
	st.Pending = st.Readings - st.Exported

	var last sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(ts) FROM measurements`).Scan(&last); err != nil {
		return st, err
	}
	if last.Valid {
		st.LastTS = &last.Int64
	}
	return st, nil
}

// Schema elenca le colonne della tabella measurements.
func (s *Store) Schema() ([]SchemaColumn, error) {
	rows, err := s.db.Query(`PRAGMA table_info(measurements)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SchemaColumn
	for rows.Next() {
		var (
			cid        int
			name, typ  string
			notnull    int
			dflt       sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &primaryKey); err != nil {
			return nil, err
		}
		out = append(out, SchemaColumn{Name: name, Type: typ})
	}
	return out, rows.Err()
}

// FromReading costruisce una Measurement da una lettura normalizzata. L'id e'
// zero perche' la riga non e' ancora stata riletta dal database.
func FromReading(r telemetry.Reading) Measurement {
	ts := r.TS.Unix()
	if r.TS.IsZero() {
		ts = time.Now().Unix()
	}
	m := Measurement{
		DeviceID:    r.DeviceID,
		DeviceLabel: r.Label,
		Topic:       r.Topic,
		Kind:        string(r.Kind),
		TS:          ts,
		WaterLevel:  r.WaterLevelCM,
		Ultrasonic:  r.UltrasonicCM,
		Tof:         r.TofCM,
		Value:       r.Value,
		PM1:         r.PM1,
		PM25:        r.PM25,
		PM10:        r.PM10,
		TempPMS:     r.TempPMS,
		HumPMS:      r.HumPMS,
		TempDHT:     r.TempDHT,
		HumDHT:      r.HumDHT,
		TempBME:     r.TempBME,
		HumBME:      r.HumBME,
		PressBME:    r.PressBME,
		TempVirt:    r.TempVirt,
		HumVirt:     r.HumVirt,
		PressVirt:   r.PressVirt,
		Lat:         r.Lat,
		Lon:         r.Lon,
		RSSI:        r.RSSI,
	}
	if r.Net != "" {
		net := r.Net
		m.Net = &net
	}
	return m
}

func scanAll(rows *sql.Rows) ([]Measurement, error) {
	defer rows.Close()
	var out []Measurement
	for rows.Next() {
		m, err := scanMeasurement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanMeasurement(rows *sql.Rows) (Measurement, error) {
	var (
		m                           Measurement
		water, us, tof, value       sql.NullFloat64
		pm1, pm25, pm10, tpms, hpms sql.NullFloat64
		tdht, hdht                  sql.NullFloat64
		tbme, hbme, pbme            sql.NullFloat64
		tvirt, hvirt, pvirt         sql.NullFloat64
		lat, lon                    sql.NullFloat64
		rssi                        sql.NullInt64
		net                         sql.NullString
		exported                    int
	)
	err := rows.Scan(
		&m.ID, &m.DeviceID, &m.DeviceLabel, &m.Topic, &m.Kind, &m.TS,
		&water, &us, &tof, &value,
		&pm1, &pm25, &pm10, &tpms, &hpms,
		&tdht, &hdht,
		&tbme, &hbme, &pbme,
		&tvirt, &hvirt, &pvirt,
		&lat, &lon, &rssi, &net, &exported,
	)
	if err != nil {
		return m, err
	}
	m.WaterLevel, m.Ultrasonic, m.Tof, m.Value = pf(water), pf(us), pf(tof), pf(value)
	m.PM1, m.PM25, m.PM10, m.TempPMS, m.HumPMS = pf(pm1), pf(pm25), pf(pm10), pf(tpms), pf(hpms)
	m.TempDHT, m.HumDHT = pf(tdht), pf(hdht)
	m.TempBME, m.HumBME, m.PressBME = pf(tbme), pf(hbme), pf(pbme)
	m.TempVirt, m.HumVirt, m.PressVirt = pf(tvirt), pf(hvirt), pf(pvirt)
	m.Lat, m.Lon = pf(lat), pf(lon)
	m.RSSI = pi(rssi)
	if net.Valid {
		m.Net = &net.String
	}
	m.Exported = exported != 0
	return m, nil
}

// UpsertDevice inserisce o aggiorna un dispositivo noto, anche prima che
// arrivi una misura. I campi vuoti non cancellano quelli gia' presenti.
func (s *Store) UpsertDevice(d Device) error {
	now := time.Now().Unix()
	if d.FirstSeen == 0 {
		d.FirstSeen = now
	}
	if d.LastSeen == 0 {
		d.LastSeen = now
	}
	_, err := s.db.Exec(`
		INSERT INTO devices (id, label, group_name, kind, topic, lat, lon, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			label      = CASE WHEN excluded.label      <> '' THEN excluded.label      ELSE devices.label      END,
			group_name = CASE WHEN excluded.group_name <> '' THEN excluded.group_name ELSE devices.group_name END,
			kind       = CASE WHEN excluded.kind       <> '' THEN excluded.kind       ELSE devices.kind       END,
			topic      = CASE WHEN excluded.topic      <> '' THEN excluded.topic      ELSE devices.topic      END,
			lat        = COALESCE(excluded.lat, devices.lat),
			lon        = COALESCE(excluded.lon, devices.lon),
			first_seen = MIN(devices.first_seen, excluded.first_seen),
			last_seen  = MAX(devices.last_seen, excluded.last_seen)`,
		d.ID, d.Label, d.Group, d.Kind, d.Topic, nf(d.Lat), nf(d.Lon), d.FirstSeen, d.LastSeen)
	return err
}

func nf(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func ni(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func ns(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func pf(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
}

func pi(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int64)
	return &i
}
