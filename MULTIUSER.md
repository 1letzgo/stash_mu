# Multi-User für Stash (Fork `1letzgo/stash_mu`, Branch `main`)

Dieser Fork ergänzt Stash um mehrere Benutzer mit Rollen. Die Rechte werden serverseitig geprüft, Bewertungen und Verlauf gibt es pro Benutzer. Die bestehende API bleibt kompatibel: `ApiKey`-Header, `?apikey=`, Session-Cookie und Login-Formular funktionieren unverändert. Clients wie stashy brauchen keine Anpassung.

## Rollen

| Rolle | Darf |
|---|---|
| **Viewer** | Inhalte durchsuchen und abspielen. Eigene Bewertungen, Wiedergaben, O-Zähler und Resume-Position. Eigene Saved Filters und UI-Einstellungen, eigener API-Key, eigenes Passwort. |
| **Editor** | Zusätzlich Metadaten bearbeiten: Szenen, Darsteller, Studios, Tags, Gruppen, Marker, Galerien, Scraper, Stash-Box-Submit. **Nicht erlaubt:** Einstellungen, Tasks (Scan, Generate, Import …), Dateien löschen, Plugins, SQL, Logs. |
| **Admin** | Alles, inklusive Benutzerverwaltung (Einstellungen → Benutzer). |

Neue GraphQL-Operationen werden standardmäßig abgelehnt (nur Admin), bis sie in `internal/api/authorization.go` eingestuft sind. Ein Test stellt sicher, dass jede Operation des Schemas eine Regel hat.

## Libraries pro Benutzer

Viewern und Editoren kann man unter Einstellungen → Benutzer einzelne Libraries (die Pfade unter Einstellungen → Library) zuweisen:

- Der Benutzer sieht nur Szenen und Bilder mit einer Datei in diesen Libraries. Galerien sind sichtbar, wenn ihr Ordner bzw. ihre Zip-Datei darin liegt.
- Marker und Kapitel folgen ihrer Szene bzw. Galerie.
- Performer, Studios, Tags und Gruppen erscheinen nur, wenn sie mit sichtbaren Inhalten verknüpft sind. Übergeordnete Studios und Tags erscheinen, wenn eine direkte Unterebene sichtbare Inhalte hat.
- Zähler und Statistiken zählen nur sichtbare Inhalte.
- Gilt überall: Listen, Suche, Einzelabruf per ID, Streams, Vorschaubilder, Bilder und Datei-/Ordnersuche. Versteckte Objekte lassen sich auch nicht per ID ändern oder löschen.
- Keine Zuweisung bedeutet Zugriff auf alle Libraries. Admins sehen immer alles.
- Szenen ohne Datei und leere manuelle Galerien sind für alle sichtbar.

Nicht eingeschränkt sind Performer, Studios, Tags und Gruppen beim Einzelabruf per ID, damit eingeschränkte Editoren neue Metadaten anlegen können.

## Was pro Benutzer gilt

- Wiedergaben, O-Zähler (Szenen und Bilder), Resume-Position und Wiedergabedauer
- Bewertungen von Szenen, Bildern, Galerien, Darstellern, Studios und Gruppen
- Saved Filters: eigene sind privat, bestehende und vom Haupt-Admin erstellte sind für alle sichtbar und nur von Admins änderbar
- UI-Einstellungen, inklusive Standardfilter. Der Haupt-Admin (ID 1) bearbeitet die globale UI-Konfiguration, die als Vorgabe für alle anderen dient.

Global bleiben: Favoriten, alle Metadaten und die Server-Einstellungen.

## Migration (Datenbank-Schema 86 → 91)

Beim ersten Start des Multi-User-Images zeigt Stash die Migrationsseite.

**Wichtig:** Den vorgeschlagenen Backup-Dateinamen stehen lassen! Mit leerem Backup-Pfad löscht Stash das Backup nach erfolgreicher Migration.

| Ausgangslage | Ergebnis nach der Migration |
|---|---|
| **Login konfiguriert** (Username/Passwort in `config.yml`) | Dieser Login wird Admin mit ID 1. Passwort, Session-Cookies und der bisherige API-Key gelten weiter. Alle bestehenden Bewertungen, Wiedergaben, O-Zähler, Resume-Werte und UI-Einstellungen gehören diesem Admin. Saved Filters bleiben für alle sichtbar. |
| **Kein Login konfiguriert** | Verhalten wie bisher: Zugriff ohne Login, alle Daten gehören dem System-User (ID 1). Der **erste** angelegte Benutzer wird automatisch Admin mit ID 1 und übernimmt diese Daten. Ab dann ist ein Login nötig. |
| **Neue Benutzer** | Starten ohne Bewertungen und ohne Verlauf. Sie sehen die geteilten Saved Filters und die globale UI-Konfiguration als Vorgabe. |

Getestet mit einer Datenbank des offiziellen Stash (Commit `aefc937`, Schema 86) mit Login + API-Key und ohne Login. Alle Werte bleiben erhalten, der alte API-Key funktioniert weiter.

### Weitere Szenarien

- **Zurück zum offiziellen Image:** Das offizielle Stash verweigert den Start mit einer migrierten Datenbank (`schema version 91 is incompatible with required schema version 86`). Für den Rückweg das Backup von vor der Migration zurückspielen. Änderungen seit der Migration gehen dabei verloren.
- **Metadaten-Export/Import:** Export und Import laufen als Admin. Exportiert werden die Bewertungen und der Verlauf **dieses** Admins. Bei einem Import mit Reset bleiben alle Benutzerkonten (inklusive Login, Rolle, Key, UI-Einstellungen) erhalten, die persönlichen Bewertungen und Verläufe anderer Benutzer aber nicht.
- **Benutzer löschen:** entfernt auch dessen Bewertungen, Verlauf, O-Zähler, Saved Filters und Einstellungen. Der Haupt-Admin (ID 1) und der letzte Admin können nicht gelöscht oder herabgestuft werden.
- **Datenbank-Backup/Restore:** Die `users`-Tabelle steckt in der Datenbank und wird mitgesichert.
- **Upstream-Update:** Neue Upstream-Migrationen ab 87 kollidieren mit 87–90 dieses Forks. Beim Rebase müssen die Fork-Migrationen hinter die Upstream-Migrationen umnummeriert werden. Das muss vor dem Update einer bestehenden Instanz passieren.

## Einschränkungen / bekannte Punkte

- DLNA hat keine Benutzer. Wiedergaben über DLNA zählen für den Haupt-Admin.
- JS-Plugins laufen ohne Benutzerkontext als Admin. Externe Plugin-Hooks laufen mit den Rechten des auslösenden Benutzers.
- Plugin-Einstellungen (`configuration.plugins`) sind für alle Benutzer lesbar, weil UI-Plugins sie im Browser brauchen. Bitte keine Geheimnisse in Plugin-Einstellungen ablegen.
- Im Web-UI sind Bearbeiten- und Löschen-Buttons für Viewer noch sichtbar. Der Server lehnt die Aktion ab, und das UI zeigt einen Fehler.

## Docker-Image

Der Workflow `.github/workflows/multiuser-image.yml` baut bei jedem Push auf `main` das Image `ghcr.io/1letzgo/stash:multi-user`. Im Fork müssen dafür GitHub Actions aktiviert sein. Das Paket ist anfangs privat: entweder auf GHCR öffentlich schalten oder auf dem Docker-Host `docker login ghcr.io` ausführen.

Testbetrieb auf Unraid als **neuer, eigener Container**:

- Image: `ghcr.io/1letzgo/stash:multi-user`
- Eigener Port, z. B. `6971:9999`
- Eigener Config-Ordner, z. B. `/mnt/user/appdata/stash-mu/config` → `/root/.stash`, mit einer **Kopie** von Config und Datenbank der Testbibliothek
- Medien-Ordner **read-only** einbinden

Die bestehenden Stash-Container bleiben unverändert.
