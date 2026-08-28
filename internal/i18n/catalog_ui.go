package i18n

// Interface chrome: navigation, key hints, panel titles, table headers.
func init() {
	register(map[string][2]string{
		// --- navigation -------------------------------------------------
		"nav.overview": {"Overview", "Genel Bakış"},
		"tab.overview": {"Overview", "Genel"},
		"tab.kernel":   {"Kernel", "Çekirdek"},
		"tab.unwall":   {"Unwall", "Unwall"},
		"nav.test":     {"Test", "Test"},
		"nav.layers":   {"Layers", "Katmanlar"},
		"nav.kernel":   {"Kernel & Boots", "Çekirdek & Açılışlar"},
		"nav.bpftune":  {"bpftune", "bpftune"},
		"nav.unwall":   {"Unwall / DPI", "Unwall / DPI"},
		"nav.dns":      {"DNS", "DNS"},
		"nav.monitor":  {"Monitor", "İzleme"},
		"nav.advice":   {"Advice", "Öneriler"},
		"nav.history":  {"History", "Geçmiş"},
		"nav.reports":  {"Reports", "Raporlar"},
		"nav.help":     {"Help", "Yardım"},

		// --- keys -------------------------------------------------------
		"key.run":      {"run", "çalıştır"},
		"key.stop":     {"stop", "durdur"},
		"key.export":   {"export", "dışa aktar"},
		"key.lang":     {"language", "dil"},
		"key.help":     {"help", "yardım"},
		"key.quit":     {"quit", "çık"},
		"key.nav":      {"navigate", "gezin"},
		"key.scroll":   {"scroll", "kaydır"},
		"key.select":   {"select suite", "paket seç"},
		"key.ab":       {"A/B compare", "A/B karşılaştır"},
		"key.baseline": {"set baseline", "referans al"},
		"key.back":     {"back", "geri"},
		"key.confirm":  {"confirm", "onayla"},

		// --- generic ----------------------------------------------------
		"ui.idle":         {"idle", "hazır"},
		"ui.running":      {"running", "çalışıyor"},
		"ui.stopped":      {"stopped", "durdurulmuş"},
		"ui.done":         {"done", "bitti"},
		"ui.cancelled":    {"cancelled", "iptal edildi"},
		"ui.notinstalled": {"not installed", "kurulu değil"},
		"ui.nodata":       {"no data yet", "henüz veri yok"},
		"ui.press_run":    {"Press r to start", "Başlatmak için r"},
		"ui.needs_root":   {"needs root", "root gerekiyor"},
		"ui.yes":          {"yes", "evet"},
		"ui.no":           {"no", "hayır"},
		"ui.on":           {"on", "açık"},
		"ui.off":          {"off", "kapalı"},
		"ui.enabled":      {"enabled", "etkin"},
		"ui.disabled":     {"disabled", "kapalı"},
		"ui.clean":        {"clean", "temiz"},
		"ui.none":         {"none", "yok"},
		"ui.unknown":      {"unknown", "bilinmiyor"},
		"ui.saved":        {"saved: %s", "kaydedildi: %s"},
		"ui.error":        {"error: %s", "hata: %s"},
		"ui.small_term":   {"Terminal too small - at least %dx%d needed.", "Terminal çok küçük - en az %dx%d gerekiyor."},
		"ui.confirm_ab":   {"Stop %s temporarily and compare? [y/n]", "%s geçici olarak durdurulup karşılaştırılsın mı? [e/h]"},
		"ui.run_first":    {"run a test first", "önce bir test çalıştır"},
		"ui.more":         {"%d more", "%d tane daha"},
		"ui.language_now": {"Language: %s", "Dil: %s"},

		// --- panels -----------------------------------------------------
		"panel.link":      {"Link", "Bağlantı"},
		"panel.kernel":    {"Kernel / TCP", "Çekirdek / TCP"},
		"panel.tools":     {"Tools", "Araçlar"},
		"panel.latency":   {"Live latency", "Canlı gecikme"},
		"panel.events":    {"Events", "Olaylar"},
		"panel.verdict":   {"Verdict", "Sonuç"},
		"panel.timeline":  {"Timeline", "Zaman çizelgesi"},
		"panel.log":       {"Event log", "Olay günlüğü"},
		"panel.stability": {"Stability", "Kararlılık"},
		"panel.boots":     {"Boot history", "Açılış geçmişi"},
		"panel.sockets":   {"Sockets", "Soketler"},
		"panel.netfilter": {"Netfilter", "Netfilter"},
		"panel.sysctl":    {"Kernel tunables", "Çekirdek ayarları"},
		"panel.changes":   {"Changes made", "Yapılan değişiklikler"},
		"panel.cc":        {"Per-connection congestion control", "Bağlantı başına tıkanıklık kontrolü"},

		// --- fields -----------------------------------------------------
		"f.iface":       {"interface", "arayüz"},
		"f.gateway":     {"gateway", "ağ geçidi"},
		"f.speed":       {"speed/mtu", "hız/mtu"},
		"f.qdisc":       {"qdisc", "qdisc"},
		"f.flaps":       {"link drops", "link düşmesi"},
		"f.counters":    {"counters", "sayaçlar"},
		"f.wifi":        {"wifi", "wifi"},
		"f.driver":      {"driver", "sürücü"},
		"f.retransmit":  {"retransmit", "yeniden gönderim"},
		"f.timeouts":    {"timeouts", "zaman aşımı"},
		"f.ooo":         {"out-of-order", "sırasız"},
		"f.sockets":     {"sockets", "soket"},
		"f.cc":          {"cong. control", "tıkanıklık kont."},
		"f.conntrack":   {"conntrack", "conntrack"},
		"f.engine":      {"engine", "motor"},
		"f.hostlist":    {"hostlist", "hostlist"},
		"f.ports":       {"ports", "portlar"},
		"f.queue":       {"queue", "kuyruk"},
		"f.dns":         {"DNS", "DNS"},
		"f.status":      {"status", "durum"},
		"f.version":     {"version", "sürüm"},
		"f.autostart":   {"autostart", "otomatik başlat"},
		"f.system":      {"system", "sistem"},
		"f.upstream":    {"upstream", "üst kaynak"},
		"f.encryption":  {"encryption", "şifreleme"},
		"f.provider":    {"provider", "sağlayıcı"},
		"f.uptime":      {"uptime", "çalışma süresi"},
		"f.outages":     {"outages", "kesinti"},
		"f.avail":       {"availability", "erişilebilirlik"},
		"f.dnsfail":     {"dns failures", "dns hatası"},
		"f.reachfail":   {"reach failures", "erişim hatası"},
		"f.lastreach":   {"last check", "son kontrol"},
		"f.bdp":         {"bandwidth-delay product", "bant-gecikme çarpımı"},
		"f.mtu":         {"MTU", "MTU"},
		"f.eee":         {"EEE", "EEE"},
		"f.nfqueue":     {"nfqueue", "nfqueue"},
		"f.gatewaymode": {"gateway mode", "ağ geçidi modu"},
		"f.score":       {"score", "puan"},
		"f.baseline":    {"baseline", "referans"},

		// --- table columns ----------------------------------------------
		"col.target":   {"target", "hedef"},
		"col.last":     {"last", "son"},
		"col.avg":      {"avg", "ort"},
		"col.p95":      {"p95", "p95"},
		"col.loss":     {"loss", "kayıp"},
		"col.jitter":   {"jitter", "jitter"},
		"col.mos":      {"MOS", "MOS"},
		"col.resolver": {"resolver", "çözücü"},
		"col.kind":     {"type", "tip"},
		"col.avgms":    {"avg ms", "ort ms"},
		"col.errors":   {"errors", "hata"},
		"col.domain":   {"domain", "alan adı"},
		"col.verdict":  {"verdict", "sonuç"},
		"col.tls":      {"tls", "tls"},
		"col.split":    {"split", "bölünmüş"},
		"col.quic":     {"quic", "quic"},
		"col.hop":      {"#", "#"},
		"col.ip":       {"ip", "ip"},
		"col.best":     {"best", "en iyi"},
		"col.worst":    {"worst", "en kötü"},
		"col.peer":     {"peer", "karşı taraf"},
		"col.rtt":      {"rtt", "rtt"},
		"col.minrtt":   {"min rtt", "min rtt"},
		"col.cwnd":     {"cwnd", "cwnd"},
		"col.retrans":  {"retrans", "retrans"},
		"col.rate":     {"rate", "hız"},
		"col.tunable":  {"tunable", "ayar"},
		"col.from":     {"from", "başlangıç"},
		"col.to":       {"now", "şu an"},
		"col.times":    {"times", "kez"},
		"col.reason":   {"reason", "gerekçe"},
		"col.boot":     {"boot", "açılış"},
		"col.kernelv":  {"kernel", "çekirdek"},
		"col.hours":    {"hours", "saat"},
		"col.drops":    {"drops", "düşme"},
		"col.perhour":  {"per hour", "saatte"},
		"col.when":     {"when", "ne zaman"},
		"col.metric":   {"metric", "ölçüt"},
		"col.delta":    {"delta", "fark"},
		"col.date":     {"date", "tarih"},
		"col.suite":    {"suite", "paket"},
		"col.grade":    {"grade", "not"},

		// --- load / speed -----------------------------------------------
		"load.idle":     {"idle", "boşta"},
		"load.download": {"download", "indirme"},
		"load.upload":   {"upload", "yükleme"},
		"load.bloat":    {"bufferbloat", "bufferbloat"},
		"load.kernel":   {"kernel: cc=%s · cwnd %.0f · socket retrans %.2f%% · minRTT %.1f ms · %d streams", "çekirdek: cc=%s · cwnd %.0f · soket retrans %%%.2f · minRTT %.1f ms · %d akış"},

		// --- sections ---------------------------------------------------
		"sec.latency":     {"Latency", "Gecikme"},
		"sec.load":        {"Under load", "Yük altında"},
		"sec.dns":         {"DNS resolvers", "DNS çözücüleri"},
		"sec.dnschecks":   {"DNS interference checks", "DNS müdahale kontrolleri"},
		"sec.dnscompare":  {"System vs encrypted resolver", "Sistem ↔ şifreli çözücü"},
		"sec.dpi":         {"DPI / reachability", "DPI / erişilebilirlik"},
		"sec.path":        {"Path", "Yol"},
		"sec.findings":    {"Findings", "Bulgular"},
		"sec.advice":      {"Advice", "Öneriler"},
		"sec.physical":    {"Physical / link", "Fiziksel / bağlantı"},
		"sec.tcpcounters": {"TCP (kernel counters)", "TCP (çekirdek sayaçları)"},
		"sec.assessment":  {"Assessment", "Değerlendirme"},
		"sec.trend":       {"Score trend", "Puan eğilimi"},

		// --- misc -------------------------------------------------------
		"misc.mtu_line":     {"MTU: interface %d · measured path %d — %s", "MTU: arayüz %d · ölçülen yol %d — %s"},
		"misc.score_line":   {"Score %.1f / 100  (%s)", "Puan %.1f / 100  (%s)"},
		"misc.derived_from": {"derived from the %s run (%s) · %d items · ordered by priority", "%s testinden (%s) türetildi · %d madde · öncelik sırasına göre"},
		"misc.priority":     {"Priority %d", "Öncelik %d"},
		"misc.gain":         {"expected gain", "beklenen kazanç"},
		"misc.risk":         {"risk", "risk"},
		"misc.revert":       {"revert", "geri alma"},
		"misc.no_runs":      {"no saved runs", "kayıtlı çalışma yok"},
		"misc.no_events":    {"no events yet", "henüz olay yok"},
		"misc.truncated":    {"log truncated", "günlük kırpılmış"},
		"misc.current":      {"current", "şu anki"},
		"misc.measured":     {"measured", "ölçülen"},
		"misc.unmeasured":   {"no log coverage", "kayıt yok"},
		"misc.per_min_loss": {"loss per minute", "dakikalık kayıp"},
		"misc.hourly":       {"outages by hour", "saate göre kesinti"},
		"misc.ab_result":    {"A/B result", "A/B sonucu"},
		"misc.vs_baseline":  {"vs baseline", "referansa göre"},
		"misc.baseline_set": {"baseline set from this run", "bu çalışma referans alındı"},
	})
}

// Monitor event wording.
func init() {
	register(map[string][2]string{
		"f.changes":        {"changes", "değişiklik"},
		"mon.started":      {"monitoring started", "izleme başladı"},
		"mon.lossburst":    {"%d consecutive packets lost", "%d ardışık paket kaybı"},
		"mon.outage.start": {"every target unresponsive", "tüm hedefler yanıtsız"},
		"mon.outage.end":   {"outage lasted %.1f seconds", "kesinti %.1f saniye sürdü"},
		"mon.spike":        {"%.0f ms (baseline %.0f ms)", "%.0f ms (taban %.0f ms)"},
		"mon.noresolve":    {"did not resolve", "çözümlenemedi"},
		"mon.flap":         {"link came back up (link flap)", "bağlantı yeniden kalktı (link flap)"},
		"mon.nfqdrop":      {"+%d dropped packets (the desync engine cannot keep up)", "+%d düşen paket (desync motoru yetişemiyor)"},
	})
}

// Labels composed from live counts.
func init() {
	register(map[string][2]string{
		"bpftune.label": {"running · %d changes", "çalışıyor · %d değişiklik"},
		"err.journal":   {"kernel log unavailable", "çekirdek günlüğü okunamadı"},
	})
}

// Keys added by the 0.3 interface.
func init() {
	register(map[string][2]string{
		"key.top":    {"top", "başa"},
		"key.bottom": {"bottom", "sona"},
	})
}

// Keys introduced by the 0.3.1 interface.
func init() {
	register(map[string][2]string{
		"nav.kernel.short": {"Kernel", "Çekirdek"},
		"nav.unwall.short": {"Unwall", "Unwall"},
		"key.top":          {"top", "başa"},
		"key.bottom":       {"bottom", "sona"},
		"ui.any_key":       {"press any key to close", "kapatmak için herhangi bir tuş"},
	})
}

// The apply/rollback flow.
func init() {
	register(map[string][2]string{
		"apply.nothing":     {"nothing to apply", "uygulanacak bir şey yok"},
		"apply.unknown":     {"unknown change id", "bilinmeyen değişiklik kimliği"},
		"apply.nosnapshots": {"no snapshots stored", "kayıtlı yedek yok"},
		"apply.available":   {"Applicable changes", "Uygulanabilir değişiklikler"},
		"apply.snapshot":    {"snapshot: %s", "yedek: %s"},
		"apply.restorehint": {"undo at any time with: nabiz rollback  (or run %s)", "istediğin an geri al: nabiz rollback  (ya da çalıştır: %s)"},
		"apply.verifying":   {"verifying connectivity…", "bağlantı doğrulanıyor…"},
		"apply.ok":          {"applied and verified", "uygulandı ve doğrulandı"},
		"apply.rolledback":  {"connectivity check failed — everything was rolled back automatically", "bağlantı kontrolü başarısız — her şey otomatik geri alındı"},
		"apply.confirm":     {"Apply these %d changes? [y/N] ", "Bu %d değişiklik uygulansın mı? [e/H] "},
		"apply.risklink":    {"link-affecting changes are excluded unless --include-link is given", "bağlantıyı etkileyen değişiklikler --include-link verilmedikçe hariç tutulur"},
		"apply.rollbackok":  {"rolled back: %s", "geri alındı: %s"},
		"apply.listempty":   {"no applicable changes in the last run", "son çalışmada uygulanabilir değişiklik yok"},
	})
}

// Buttons and dialogs.
func init() {
	register(map[string][2]string{
		"ui.cancel": {"Cancel", "Vazgeç"},
		"ui.start":  {"Start", "Başlat"},
		"ui.apply":  {"Apply", "Uygula"},
		"ui.close":  {"Close", "Kapat"},
	})
}

// The in-interface approval flow.
func init() {
	register(map[string][2]string{
		"apply.selected":     {"%d selected", "%d seçili"},
		"apply.rollback":     {"Rollback", "Geri al"},
		"apply.select_safe":  {"Select safe", "Güvenlileri seç"},
		"apply.clear":        {"Clear", "Temizle"},
		"apply.risk.low":     {"low", "düşük"},
		"apply.risk.medium":  {"medium", "orta"},
		"apply.risk.link":    {"link", "bağlantı"},
		"apply.dialog.title": {"Apply %d changes", "%d değişiklik uygulanacak"},
		"apply.dialog.body": {
			"A snapshot is written first and an undo script is generated from your current values.\n\nAfter applying, connectivity is verified (ping + DNS + TLS). If it fails, everything is rolled back automatically.\n\nYou will be asked for your password once.\n\n%s",
			"Önce yedek alınır ve mevcut değerlerinden bir geri alma betiği üretilir.\n\nUygulandıktan sonra bağlantı doğrulanır (ping + DNS + TLS). Başarısız olursa her şey otomatik geri alınır.\n\nŞifren bir kez sorulacak.\n\n%s"},
		"apply.dialog.rollback": {"Undo the last applied batch?\n\n%s", "Son uygulanan değişiklikler geri alınsın mı?\n\n%s"},
		"apply.running":         {"applying…", "uygulanıyor…"},
		"apply.needs_pkexec": {
			"pkexec was not found. Running this from a full-screen interface needs a graphical password prompt; use `nabiz apply` in a terminal instead.",
			"pkexec bulunamadı. Tam ekran arayüzden çalıştırmak grafik şifre istemi gerektirir; bunun yerine terminalde `nabiz apply` kullan."},
	})
}

// Post-apply reporting.
func init() {
	register(map[string][2]string{
		"apply.remaining": {"%d changes still applicable", "%d değişiklik hâlâ uygulanabilir"},
		"apply.recheck":   {"re-reading the machine…", "makine yeniden okunuyor…"},
	})
}

// Advice action bar.
func init() {
	register(map[string][2]string{
		"apply.select_all": {"Select all", "Tümünü seç"},
	})
}
