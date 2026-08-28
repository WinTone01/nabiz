package i18n

// Chrome added by the interface rewrite: navigation groups, the command
// palette, and the words the status rows and dialogs need.
func init() {
	register(map[string][2]string{
		// --- navigation groups -------------------------------------------
		"nav.group.live":    {"Live", "Canlı"},
		"nav.group.test":    {"Measure", "Ölçüm"},
		"nav.group.system":  {"System", "Sistem"},
		"nav.group.results": {"Results", "Sonuçlar"},

		// --- keys ---------------------------------------------------------
		"key.move":    {"move", "gez"},
		"key.page":    {"page", "sayfa"},
		"key.focus":   {"focus", "odak"},
		"key.toggle":  {"toggle", "işaretle"},
		"key.refresh": {"refresh", "yenile"},
		"key.palette": {"commands", "komutlar"},

		// --- command palette ------------------------------------------------
		"ui.palette.title":       {"Commands", "Komutlar"},
		"ui.palette.placeholder": {"Type to filter…", "Süzmek için yaz…"},
		"ui.palette.empty":       {"No command matches.", "Eşleşen komut yok."},

		// --- status and feedback ----------------------------------------------
		"ui.refreshed":    {"Environment re-read.", "Ortam yeniden okundu."},
		"ui.not_here":     {"Nothing to measure on this screen.", "Bu ekranda ölçülecek bir şey yok."},
		"ui.confirm_quit": {"Stop the monitor and quit?", "İzleme durdurulup çıkılsın mı?"},
		"ui.help_hint": {
			"Press ctrl+k for every command by name, or a number key to jump straight to a section.",
			"Her komuta adıyla ulaşmak için ctrl+k, bir bölüme doğrudan gitmek için rakam tuşlarına bas.",
		},
		"misc.last_run": {"last run", "son çalışma"},
		"sec.score":     {"Score", "Puan"},
		"tile.latency":  {"Internet", "İnternet"},
		"tile.link":     {"Link", "Bağlantı"},
		"tile.drops":    {"drops", "düşme"},

		// --- palette entries ---------------------------------------------------
		"cmd.section.go":     {"go", "git"},
		"cmd.section.run":    {"run", "çalıştır"},
		"cmd.section.action": {"action", "eylem"},

		"cmd.goto":      {"Go to %s", "%s bölümüne git"},
		"cmd.run_suite": {"Run the %s suite", "%s paketini çalıştır"},

		"cmd.run_current": {"Run this screen's suite", "Bu ekranın paketini çalıştır"},
		"cmd.stop":        {"Stop the run", "Çalışmayı durdur"},
		"cmd.export":      {"Export the last run", "Son çalışmayı dışa aktar"},
		"cmd.baseline":    {"Pin the last run as baseline", "Son çalışmayı referans olarak sabitle"},
		"cmd.ab":          {"A/B compare on this screen", "Bu ekranda A/B karşılaştır"},
		"cmd.apply":       {"Apply the selected advice", "Seçili önerileri uygula"},
		"cmd.rollback":    {"Roll back the last applied change", "Son uygulanan değişikliği geri al"},
		"cmd.refresh":     {"Re-read the environment", "Ortamı yeniden oku"},
		"cmd.lang":        {"Switch the language to %s", "Dili %s olarak değiştir"},
		"cmd.help":        {"Show the key sheet", "Tuş listesini göster"},
		"cmd.quit":        {"Quit", "Çık"},
	})
}
