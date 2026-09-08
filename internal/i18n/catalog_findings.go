package i18n

// Findings: one .title (with printf args) and an optional .hint per key.
func init() {
	register(map[string][2]string{
		// --- physical link ----------------------------------------------
		"fnd.link-speed.title":         {"%s negotiated at %d Mbit/s", "%s %d Mbit/s ile anlaşmış"},
		"fnd.link-speed.hint":          {"if you expect gigabit, the cable or the switch port is the limit", "gigabit bekliyorsan kablo veya switch portu sınırlıyor demektir"},
		"fnd.duplex.title":             {"%s duplex - collisions and loss are unavoidable", "%s duplex - çarpışma ve kayıp kaçınılmaz"},
		"fnd.carrier-flaps.title":      {"link came back up %d times (link flap)", "bağlantı %d kez yeniden kalktı (link flap)"},
		"fnd.carrier-flaps.hint":       {"every flap is a few seconds of total outage", "her flap birkaç saniyelik tam kopma demektir"},
		"fnd.link-drops.title":         {"link dropped %d times", "bağlantı %d kez düştü"},
		"fnd.link-drops.span":          {", over %.0f min, one every %.1f min", ", %.0f dakikada, ortalama %.1f dk arayla"},
		"fnd.link-drops.down":          {"; %.0f s offline in total (%.1f%%), longest %.0f s", "; toplam %.0f sn kopuk (%%%.1f), en uzunu %.0f sn"},
		"fnd.link-drops.hint":          {"each drop kills every open connection: calls and games go down with it", "her düşüş tam kopma demektir: açık tüm bağlantılar kopar, oyun ve görüşme düşer"},
		"fnd.link-drops.quiet":        {"; but no drop for the last %.0f min", "; ancak son %.0f dakikadır hiç düşme yok"},
		"fnd.link-drops.quiet-hint":   {"the count covers the whole boot and cannot fall, so it still carries the old rate; the quiet stretch is what says the link is behaving now", "sayaç tüm açılışı kapsar ve düşemez, o yüzden eski hızı taşımaya devam eder; şu an linkin düzgün olduğunu söyleyen şey sessiz geçen süredir"},
		"fnd.link-downshift.title":     {"kernel reported downshift %d times: %s", "çekirdek %d kez downshift bildirdi: %s"},
		"fnd.link-downshift.hint":      {"gigabit negotiation keeps failing so the speed is reduced. 1000BASE-T uses all four pairs, 100BASE-TX only two - classically a broken pair, but a driver regression produces the same message", "gigabit anlaşması tutmadığı için hız düşürülüyor. 1000BASE-T dört çiftin dördünü, 100BASE-TX yalnızca ikisini kullanır - klasik olarak kopuk bir çift demektir, ama sürücü regresyonu da aynı mesajı üretir"},
		"fnd.link-regression.title":    {"link drops started recently: %s", "düşmeler yakın zamanda başladı: %s"},
		"fnd.link-regression.hint":     {"a problem that started recently points at a new variable rather than a cable that has worked for years", "yakın zamanda başlayan bir sorun, yıllardır çalışan bir kablodan çok yeni bir değişkeni işaret eder"},
		"fnd.link-regression.window":   {"a previous boot logged no drops for %.0f minutes while this one drops %.1f times an hour", "önceki bir açılışta %.0f dakika boyunca hiç düşme yokken şimdi saatte %.1f düşme var"},
		"fnd.link-regression.worse":    {"the previous boot dropped %.1f times an hour, this one %.1f", "önceki açılışta saatte %.1f düşme varken şimdi %.1f"},
		"fnd.kernel-regression.title":  {"kernel regression: %s", "çekirdek regresyonu: %s"},
		"fnd.kernel-regression.hint":   {"same cable, same modem, different kernel - the fault is on the driver side", "aynı kablo, aynı modem, farklı çekirdek - sorunun kaynağı sürücü tarafında"},
		"fnd.kernel-regression.detail": {"%s ran for %.0f h with no drops on record; %s drops %.1f times an hour", "%s ile toplam %.0f saat çalışılmış ve kayıtlarda düşme yok; %s ile saatte %.1f düşme var"},
		"fnd.eee-active.title":         {"Energy Efficient Ethernet is active while the link keeps dropping", "Energy Efficient Ethernet etkin ve bağlantı düşüyor"},
		"fnd.eee-active.hint":          {"on drivers such as r8169, EEE combined with marginal cabling is a known cause of link flapping", "r8169 gibi sürücülerde EEE, sınırda kablolarla birlikte link flap'e yol açar"},
		"fnd.eee-unknown.title":       {"Energy Efficient Ethernet state could not be read (%s) while the link keeps dropping", "Bağlantı düşerken Energy Efficient Ethernet durumu okunamadı (%s)"},
		"fnd.eee-unknown.hint":        {"EEE is one of the first things to suspect on a flapping link; an unread setting is not an absent one. Install ethtool, or run this with sudo, and look again", "Düşen bir bağlantıda ilk şüphelilerden biri EEE'dir; okunamayan bir ayar yok sayılamaz. ethtool kur veya sudo ile çalıştırıp tekrar bak"},
		"fnd.aspm-blocked.title":       {"%s could not disable PCIe power saving for %s: the firmware kept ASPM control", "%s, %s için PCIe güç tasarrufunu kapatamadı: ASPM kontrolü firmware'de kaldı"},
		"fnd.aspm-blocked.hint":        {"the driver disables ASPM on this chip on purpose, so the card is now running with exactly the power saving its own driver objected to - a known source of link drops on Realtek NICs", "sürücü bu yongada ASPM'i bilerek kapatır; yani kart, kendi sürücüsünün itiraz ettiği güç tasarrufuyla çalışıyor - Realtek kartlarda bilinen bir link düşmesi kaynağı"},
		"fnd.aspm-unverified.title":    {"%s could not disable PCIe power saving, and whether it is actually on cannot be read without root", "%s PCIe güç tasarrufunu kapatamadı; gerçekten açık olup olmadığı root olmadan okunamıyor"},
		"fnd.aspm-unverified.hint":     {"firmware that never enabled ASPM refuses the same call, so the message alone decides nothing; run sudo nabiz deep, or lspci -vv, and look for the LnkCtl line", "ASPM'i hiç açmamış bir firmware de aynı isteği reddeder, yani mesaj tek başına bir şey söylemez; sudo nabiz deep veya lspci -vv ile LnkCtl satırına bakın"},
		"fnd.journal-volatile.title":   {"the journal does not survive reboots, so only %d boots can be read", "journal yeniden başlatmayı atlatmıyor, bu yüzden yalnızca %d açılış okunabiliyor"},
		"fnd.journal-volatile.hint":    {"earlier kernels cannot be compared: with no stored log they score zero drops out of zero observed minutes, which reads as flawless", "önceki çekirdekler karşılaştırılamaz: kayıt olmayınca sıfır gözlenen dakikada sıfır düşme çıkar ve bu kusursuz görünür"},
		"fnd.wifi-signal.title":        {"signal %.0f dBm - weak", "sinyal %.0f dBm - zayıf"},
		"fnd.wifi-signal.hint":         {"expect loss and jitter; move to 5 GHz or closer to the access point", "kayıp ve jitter beklenir; 5 GHz'e geçin veya erişim noktasına yaklaşın"},
		"fnd.nic-errors.title":         {"%s = %d (%.4f%% of packets)", "%s = %d (%%%.4f paket)"},
		"fnd.nic-errors.crc":           {"a CRC error means the packet was corrupted on the wire - software cannot fix it", "CRC hatası, paketin kablo üzerinde bozulduğu anlamına gelir - yazılımla düzeltilemez"},
		"fnd.nic-errors.fifo":          {"the card cannot keep up: netdev_max_backlog or the driver queue may be too small", "kart paketleri işleyemiyor: netdev_max_backlog veya sürücü kuyruğu küçük olabilir"},
		"fnd.nic-errors.drop":          {"normal on virtual interfaces (docker, tailscale); look closer above 0.01%", "sanal arayüzlerde (docker, tailscale) normal olabilir; oran %0,01'i geçerse bakın"},
		"fnd.qdisc-drops.title":        {"the %s queue dropped %d packets", "%s kuyruğu %d paket düşürdü"},
		"fnd.qdisc-drops.hint":         {"normal for fq_codel/cake: it drops deliberately to keep the queue short", "fq_codel/cake için normaldir: kuyruğu kısa tutmak için bilerek düşürür"},

		// --- kernel counters --------------------------------------------
		"fnd.tcp-retrans.title":  {"%.2f%% TCP retransmission kernel-wide (%d/%d segments)", "çekirdek genelinde %%%.2f TCP yeniden gönderim (%d/%d segment)"},
		"fnd.tcp-retrans.hint":   {"loss from the physical layer or from congestion: if the ping to the modem is clean, it is the latter", "kayıp fiziksel katmandan mı tıkanıklıktan mı: modeme ping temizse ikincisi"},
		"fnd.tcp-timeouts.title": {"%d TCP timeouts", "%d TCP zaman aşımı"},
		"fnd.tcp-timeouts.hint":  {"each one is at least 200 ms of stall", "her biri en az 200 ms donma demektir"},
		"fnd.tcp-ofo.title":      {"%d out-of-order packets", "%d sırasız paket"},
		"fnd.tcp-ofo.hint":       {"packet order is being disturbed on the path; some desync strategies produce this too", "yol üzerinde paket sırası bozuluyor; bazı desync stratejileri de bunu üretir"},
		"fnd.conntrack.title":    {"conntrack table %.0f%% full", "conntrack tablosu %%%.0f dolu"},
		"fnd.conntrack.hint":     {"once it fills, new connections are dropped silently", "tablo dolduğunda yeni bağlantılar sessizce düşer"},

		// --- latency -----------------------------------------------------
		"fnd.loss.title":         {"%.1f%% packet loss on the internet side (%s)", "internet tarafında %%%.1f paket kaybı (%s)"},
		"fnd.loss.hint":          {"if the loss starts past the modem it is a line or ISP problem", "kayıp modemden sonra başlıyorsa hat/ISS sorunudur"},
		"fnd.loss-small.title":   {"%.1f%% packet loss (%s)", "%%%.1f paket kaybı (%s)"},
		"fnd.jitter.title":       {"jitter %.1f ms - calls and games will suffer", "jitter %.1f ms - sesli görüşme ve oyun bozulur"},
		"fnd.jitter-small.title": {"jitter %.1f ms", "jitter %.1f ms"},
		"fnd.loss-burst.title":   {"%d consecutive packets lost - short outages", "arka arkaya %d paket kaybı - kısa kopmalar var"},
		"fnd.loss-burst.hint":    {"if the link-drop counter is also rising, the cause is the physical layer", "link düşme sayacı da artıyorsa sebep fiziksel katmandır"},
		"fnd.spikes.title":       {"latency spikes: avg %.0f ms, p95 %.0f ms", "gecikme sıçramaları: ort %.0f ms, p95 %.0f ms"},
		"fnd.rtt.title":          {"baseline latency %.0f ms - high", "temel gecikme %.0f ms - yüksek"},
		"fnd.rtt.hint":           {"unless this is a mobile or satellite link, the route may be longer than it needs to be", "mobil/uydu bağlantı değilse yönlendirme gereğinden uzun olabilir"},
		"fnd.lan-loss.title":     {"%.1f%% loss on the way to the modem", "modeme giden yolda %%%.1f kayıp"},
		"fnd.lan-loss.hint":      {"this is not the ISP: it is the cable, the Wi-Fi or a switch inside your home", "sorun ISS'de değil, ev içi bağlantıda: kablo, Wi-Fi veya switch"},
		"fnd.lan-jitter.title":   {"jitter %.1f ms on the local network", "yerel ağda jitter %.1f ms"},

		// --- path ---------------------------------------------------------
		"fnd.path-loss.title": {"loss starts at hop %d (%s)", "%d. atlamadan (%s) itibaren kayıp başlıyor"},
		"fnd.path-loss.hint":  {"everything past this point is the ISP's responsibility - send them the report", "bu noktadan sonrası ISS'nin sorumluluğunda - raporu onlara gönderin"},
		"fnd.pmtu.hint":       {"setting tcp_mtu_probing=1 or lowering the interface MTU fixes it", "tcp_mtu_probing=1 yapmak veya arayüz MTU'sunu düşürmek çözer"},

		// --- dns -----------------------------------------------------------
		"fnd.dns-fail.title":            {"%s never answered", "%s hiç yanıt vermedi"},
		"fnd.dns-slow.title":            {"%s averages %.0f ms - slow", "%s ortalama %.0f ms - yavaş"},
		"fnd.dns-encryption-cost.title": {"encrypted DNS costs +%.0f ms on the first query (%.0f → %.0f ms)", "şifreli DNS ilk sorguda +%.0f ms getiriyor (%.0f → %.0f ms)"},
		"fnd.dns-encryption-cost.hint":  {"a resolver that keeps the connection open (dnscrypt-proxy, DoT) closes most of that gap, and the difference disappears once the cache is warm", "kalıcı bağlantı tutan dnscrypt-proxy/DoT bu farkı büyük ölçüde kapatır; önbellek ısındıktan sonra fark kaybolur"},
		"fnd.dns-mismatch.title":        {"the system and the encrypted resolver return different IPs for %s", "%s için sistem ve şifreli çözücü farklı IP veriyor"},
		"fnd.dns-mismatch.hint":         {"normal for CDNs; suspect redirection only if the same name also fails the TLS probe", "CDN'lerde normaldir; aynı alan adı TLS testinde de patlıyorsa yönlendirme şüphesi"},

		// --- dpi ------------------------------------------------------------
		"fnd.dpi-split.title":    {"%d domains only open when the ClientHello is split (%s)", "%d alan adı yalnızca ClientHello bölündüğünde açılıyor (%s)"},
		"fnd.dpi-split.hint":     {"the DPI matches SNI inside a single packet: zapret's multisplit/multidisorder strategies exist for exactly this", "DPI SNI'yi tek pakette arıyor: zapret'in multisplit/multidisorder stratejileri tam bunun için"},
		"fnd.dpi-blocked.title":  {"%d domains did not open by any method (%s)", "%d alan adı hiçbir yöntemle açılmadı (%s)"},
		"fnd.dpi-blocked.hint":   {"if zapret is running, the strategy may not match this ISP; search again with blockcheck", "zapret çalışıyorsa strateji bu ISS'ye uymuyor olabilir; blockcheck ile yeniden arayın"},
		"fnd.dpi-dns.title":      {"%d domains did not resolve (%s)", "%d alan adı çözümlenemedi (%s)"},
		"fnd.dpi-dns.hint":       {"the hostlist may contain names that no longer exist", "hostlist'te artık var olmayan alan adları olabilir"},
		"fnd.quic-blocked.title": {"QUIC (UDP/443) does not get through to any target", "UDP/443 (QUIC) hiçbir hedefte geçmiyor"},
		"fnd.quic-blocked.hint":  {"the browser silently falls back to TCP; check zapret's UDP rules or the ISP filter", "tarayıcı sessizce TCP'ye düşer; zapret'in UDP kurallarını veya ISS filtresini kontrol edin"},
		"fnd.dpi-clean.title":    {"every tested domain opened directly", "test edilen alan adlarının hepsi doğrudan açıldı"},

		// --- load ------------------------------------------------------------
		"fnd.bufferbloat-bad.title": {"latency rises by +%.0f ms under load (grade %s)", "yük altında gecikme +%.0f ms artıyor (not %s)"},
		"fnd.bufferbloat-bad.hint":  {"the modem/router queue is swelling: SQM (cake/fq_codel) or rate limiting fixes it", "modem/router kuyruğu şişiyor: SQM (cake/fq_codel) veya hız sınırlama ile düzelir"},
		"fnd.bufferbloat-ok.title":  {"latency only rises by +%.0f ms under load (grade %s)", "yük altında gecikme yalnızca +%.0f ms (not %s)"},
		"fnd.speed-error.title":     {"download test: %s", "indirme testi: %s"},
		"fnd.speed-gap.title":       {"download %.0f Mbps against a %d Mbit/s interface", "indirme %.0f Mbps, arayüz %d Mbit/s"},
		"fnd.load-retrans.title":    {"%.1f%% per-socket retransmission during the download", "indirme sırasında soket başına %%%.1f yeniden gönderim"},
		"fnd.load-retrans.hint":     {"congestion or line quality; the gap between cwnd and min_rtt tells you which", "tıkanıklık veya hat kalitesi; cwnd ve min_rtt farkı hangisi olduğunu söyler"},
		"fnd.cc-mixed.title":        {"several congestion control algorithms were used under load: %s", "yük sırasında birden fazla tıkanıklık algoritması kullanıldı: %s"},
		"fnd.cc-mixed.hint":         {"bpftune may be choosing per socket", "bpftune soket bazında seçim yapıyor olabilir"},

		// --- unwall / netfilter -------------------------------------------
		"fnd.unwall-engine.title":       {"the service is running but the engine binary is missing - traffic is not processed", "servis çalışıyor ama motor ikilisi yok - trafik işlenmiyor"},
		"fnd.unwall-engine.hint":        {"build the engine with unwallctl build", "unwallctl build ile motoru derleyin"},
		"fnd.unwall-nft.title":          {"the nftables table is not visible - packets never reach NFQUEUE", "nftables tablosu görünmüyor - paketler NFQUEUE'ya girmiyor"},
		"fnd.nfqueue-drop.title":        {"NFQUEUE dropped %d packets", "NFQUEUE %d paket düşürdü"},
		"fnd.nfqueue-drop.hint":         {"the desync engine cannot keep up: narrow the hostlist, turn off gateway mode, or raise the queue length", "desync motoru trafiğe yetişemiyor: hostlist'i daraltın, gateway modunu kapatın veya kuyruk uzunluğunu artırın"},
		"fnd.unwall-autohostlist.title": {"autohostlist has grown to %d domains", "autohostlist %d alan adına ulaşmış"},
		"fnd.unwall-autohostlist.hint":  {"false positives push unrelated traffic through desync; pruning the list lowers latency and CPU load", "yanlış pozitifler gereksiz trafiği desync'e sokar; listeyi temizlemek gecikmeyi ve CPU yükünü düşürür"},
		"fnd.unwall-gateway.title":      {"gateway mode is on - this machine also carries LAN traffic", "ağ geçidi modu açık - bu makine LAN trafiğini de taşıyor"},
		"fnd.unwall-gateway.hint":       {"turning it off lowers conntrack and CPU load if you are not using it", "kullanmıyorsanız kapatmak conntrack ve CPU yükünü azaltır"},
		"fnd.unwall-quic.title":         {"UDP/443 (QUIC) passes through the engine", "UDP/443 (QUIC) motordan geçiyor"},
		"fnd.unwall-quic.hint":          {"removing 443 from PORTS_UDP is the first thing to try when QUIC misbehaves", "QUIC sorunlarında PORTS_UDP'den 443'ü çıkarmak ilk denenecek adımdır"},
		"fnd.dns-leak.title":            {"encrypted DNS is on but plaintext resolvers are still configured: %s", "şifreli DNS açık ama düz metin çözücüler hâlâ yapılandırılmış: %s"},
		"fnd.dns-leak.hint":             {"clear FallbackDNS in resolved.conf or review DNSStubListener", "resolved.conf içinde FallbackDNS satırını temizleyin veya DNSStubListener ayarını gözden geçirin"},
		"fnd.nfqueue-root.title":        {"NFQUEUE counters cannot be read without root", "NFQUEUE sayaçları root olmadan okunamıyor"},
		"fnd.nfqueue-root.hint":         {"run sudo nabiz deep to see whether the desync engine is dropping packets", "sudo nabiz deep ile çalıştırırsan desync motorunun paket düşürüp düşürmediği görünür"},

		// --- bpftune --------------------------------------------------------
		"fnd.bpftune-stopped.title":      {"bpftune is installed but not running; the values it wrote stay until reboot", "bpftune kurulu ama çalışmıyor; yazdığı değerler yeniden başlatana kadar kalır"},
		"fnd.bpftune-off-by-nabiz.title": {"bpftune is stopped because nabiz stopped it - this is not a fault", "bpftune duruyor çünkü onu nabiz durdurdu - bu bir arıza değil"},
		"fnd.bpftune-off-by-nabiz.hint":  {"the tuner-off change disabled the unit; roll that batch back to bring it running again", "ayarlayıcıyı kapat değişikliği birimi devre dışı bıraktı; tekrar çalışması için o partiyi geri al"},
		"fnd.bpftune-idle.title":         {"bpftune is running but has not changed anything yet", "bpftune çalışıyor ama henüz hiçbir ayarı değiştirmemiş"},
		"fnd.bpftune-buffers-high.title": {"tcp_rmem ceiling is %s, %.0f× this line's BDP%s", "tcp_rmem tavanı %s, bu hattın BDP'sinin %.0f katı%s"},
		"fnd.bpftune-buffers-high.hint":  {"%d Mbit/s × %.0f ms ≈ %s can be in flight; a bigger buffer does not raise throughput, it adds queueing and latency under congestion. Pin it with: sysctl -w net.ipv4.tcp_rmem=\"4096 131072 %d\"", "%d Mbit/s × %.0f ms ≈ %s veri yolda olabilir; daha büyük tampon hızı artırmaz, tıkanıklıkta kuyruk ve gecikme yaratır. Sabitlemek için: sysctl -w net.ipv4.tcp_rmem=\"4096 131072 %d\""},
		"fnd.bpftune-buffers-ok.title":   {"tcp_rmem ceiling is %s, %.0f× the BDP - reasonable%s", "tcp_rmem tavanı %s, BDP'nin %.0f katı - makul%s"},
		"fnd.bpftune-buffers-info.title": {"tcp_rmem ceiling is %s (BDP %s)", "tcp_rmem tavanı %s (BDP %s)"},
		"fnd.bpftune-buffers.origin":     {" (bpftune raised %s → %s)", " (bpftune %s → %s yaptı)"},
		"fnd.bpftune-cc.title":           {"choosing congestion control per connection: %s", "bağlantı başına tıkanıklık algoritması seçiyor: %s"},
		"fnd.bpftune-cc.hint":            {"which is why a single tcp_congestion_control value is misleading - the real choice is made per socket", "bu yüzden tek bir 'tcp_congestion_control' değerine bakmak yanıltıcıdır; gerçek seçim soket bazında yapılıyor"},
		"fnd.bpftune-cc-allowed.title":   {"widened the list of allowed congestion control algorithms", "izin verilen tıkanıklık algoritmaları listesini genişletti"},
		"fnd.bpftune-cc-allowed.hint":    {"dctcp only makes sense where ECN works end to end; over the internet it mostly behaves like reno", "dctcp yalnızca ECN'i uçtan uca destekleyen ağlarda anlamlıdır; internet üzerinden çoğunlukla reno gibi davranır"},
		"fnd.bpftune-retrans.title":      {"%.2f%% retransmission kernel-wide", "çekirdek genelinde %%%.2f yeniden gönderim"},
		"fnd.bpftune-retrans.hint":       {"bpftune tends to grow buffers in this situation; if the loss comes from the physical layer, a bigger buffer hides the problem instead of fixing it", "bpftune bu koşulda tamponları büyütme eğilimindedir; oysa kayıp fiziksel katmandan geliyorsa büyük tampon sorunu gizler, çözmez"},

		// --- sysctl ----------------------------------------------------------
		"fnd.sysctl.qdisc":    {"no queue management: fq_codel or cake lowers bufferbloat", "kuyruk yönetimi yok: fq_codel veya cake bufferbloat'ı düşürür"},
		"fnd.sysctl.cubic":    {"cubic; on lossy links bbr can improve upload stability", "cubic; kayıplı hatlarda bbr yükleme kararlılığını artırabilir"},
		"fnd.sysctl.wscale":   {"disabled - throughput is capped", "kapalı - hız tavanlanır"},
		"fnd.sysctl.sack":     {"disabled - loss recovery gets worse", "kapalı - kayıp toparlanması kötüleşir"},
		"fnd.sysctl.nopmtu":   {"PMTU discovery is disabled", "PMTU keşfi kapalı"},
		"fnd.sysctl.mtuprobe": {"MTU is %d and MTU probing is off - blackhole risk", "MTU %d ve MTU probing kapalı - kara delik riski"},
		"fnd.sysctl.rmem":     {"very large buffer (%d) - can add latency", "çok büyük tampon (%d) - gecikmeyi artırabilir"},

		// --- generic ---------------------------------------------------------
		"fnd.clean.title": {"No significant problem found.", "Belirgin bir sorun bulunamadı."},

		// --- dns checks --------------------------------------------------------
		"chk.transparent-dns.bad":  {"UDP/53 is intercepted: %s answered (%s)", "UDP/53 ele geçirilmiş: %s yanıt verdi (%s)"},
		"chk.transparent-dns.ok":   {"no UDP/53 redirection", "UDP/53 yönlendirmesi yok"},
		"chk.nxdomain-hijack.bad":  {"a non-existent domain was redirected to %s", "var olmayan alan adı %s adresine yönlendirildi"},
		"chk.dnssec.warn":          {"not validating (a deliberately broken zone resolved)", "doğrulama yapılmıyor (bozuk imzalı bölge çözüldü)"},
		"chk.dnssec.ok":            {"validating (AD=1)", "doğrulanıyor (AD=1)"},
		"chk.udp-vs-tcp.ok":        {"same answer", "aynı yanıt"},
		"chk.udp-vs-tcp.tcpfail":   {"TCP/53 does not work: large answers and DNSSEC may break", "TCP/53 çalışmıyor: büyük yanıtlar ve DNSSEC bozulabilir"},
		"chk.udp-vs-tcp.udpfail":   {"UDP/53 does not work while TCP does", "UDP/53 çalışmıyor, TCP çalışıyor"},
		"chk.udp-vs-tcp.truncated": {"the UDP answer is truncated (TC bit)", "UDP yanıtı kırpılıyor (TC biti)"},
		"chk.injection.ok":         {"a single answer arrived", "tek yanıt geldi"},
		"chk.injection.bad":        {"more than one answer for the same query - injection", "aynı sorguya birden fazla yanıt - enjeksiyon"},
		"chk.edns.ok":              {"EDNS0 passes cleanly", "EDNS0 sorunsuz"},
		"chk.edns.warn":            {"EDNS0/DO query failed: %s", "EDNS0/DO sorgusu başarısız: %s"},
		"chk.edns.info":            {"a large answer was truncated, falling back to TCP", "büyük yanıt kırpıldı, TCP'ye düşülüyor"},
	})
}

// Extra check phrasings that need their own arguments.
func init() {
	register(map[string][2]string{
		"chk.udp-vs-tcp.differ": {"UDP and TCP disagree for %s (udp %s / tcp %s)", "%s için UDP ve TCP farklı yanıt veriyor (udp %s / tcp %s)"},
		"chk.injection.two":     {"two different answers to the same query: %s (%.0f ms) and %s (%.0f ms) - injection", "aynı soruya iki farklı yanıt: %s (%.0f ms) ve %s (%.0f ms) - enjeksiyon"},
		"chk.injection.dupe":    {"%d duplicate answers, identical content", "%d yinelenen yanıt, içerik aynı"},
	})
}

// Findings added in 0.3.
func init() {
	register(map[string][2]string{
		"fnd.fw-icmp.title":      {"the firewall dropped %d 'fragmentation needed' ICMP messages", "güvenlik duvarı %d adet 'fragmentation needed' ICMP mesajını düşürdü"},
		"fnd.fw-icmp.hint":       {"blocking that message breaks path MTU discovery: connections open and then stall on the first large packet", "bu mesajı engellemek yol MTU keşfini bozar: bağlantı kurulur ama ilk büyük pakette donar"},
		"fnd.ipv6-broken.title":  {"IPv6 is configured but nothing answers over it", "IPv6 yapılandırılmış ama üzerinden hiçbir şey yanıt vermiyor"},
		"fnd.ipv6-broken.hint":   {"applications try IPv6 first and wait for the timeout on every new connection", "uygulamalar önce IPv6 deneyip her yeni bağlantıda zaman aşımını bekler"},
		"fnd.ipv6-missing.title": {"no IPv6 connectivity, although DNS returns AAAA records", "IPv6 bağlantısı yok, buna karşın DNS AAAA kaydı döndürüyor"},
	})
}

// A unit that tried to start and died is not the same as one that is off.
func init() {
	register(map[string][2]string{
		"fnd.bpftune-failed.title":    {"bpftune failed to start (%s)", "bpftune başlatılamadı (%s)"},
		"fnd.bpftune-failed.override": {"an override in /etc/systemd/system/bpftune.service.d is replacing the packaged ExecStart; removing it restores the unit", "/etc/systemd/system/bpftune.service.d içindeki bir geçersiz kılma paketin ExecStart satırını değiştiriyor; onu kaldırmak birimi eski hâline getirir"},
	})
}
