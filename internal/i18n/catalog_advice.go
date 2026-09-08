package i18n

// Advice strings. Shell commands stay untranslated in the code; only the prose
// steps live here.
func init() {
	register(map[string][2]string{
		// --- categories ---------------------------------------------------
		"cat.physical": {"physical", "fiziksel"},
		"cat.queue":    {"queue", "kuyruk"},
		"cat.kernel":   {"kernel", "çekirdek"},
		"cat.bpftune":  {"bpftune", "bpftune"},
		"cat.dns":      {"dns", "dns"},
		"cat.dpi":      {"dpi", "dpi"},
		"cat.isp":      {"isp", "iss"},
		"cat.app":      {"application", "uygulama"},
		"cat.security": {"security", "güvenlik"},
		"cat.method":   {"method", "yöntem"},

		// --- kernel regression / downgrade ---------------------------------
		"adv.kernel-downgrade.title":   {"Roll the kernel back to %s", "Çekirdeği %s sürümüne geri al"},
		"adv.kernel-downgrade.why":     {"%s. The cable, the modem and the settings did not change - only the kernel did, and the drops start in the very first session booted on the new one.", "%s. Kablo, modem ve ayarlar aynıyken yalnızca çekirdek değişti; düşmeler tam da yeni çekirdekle açılan ilk oturumda başladı."},
		"adv.kernel-downgrade.nocache": {"The old kernel packages are not in the cache; try booting linux-cachyos-lts instead (already installed)", "Eski çekirdek paketleri önbellekte yok; linux-cachyos-lts ile açmayı dene (kurulu)"},
		"adv.kernel-downgrade.reboot":  {"Reboot and use the machine for an hour", "Yeniden başlat ve bir saat kullan"},
		"adv.kernel-downgrade.verify":  {"Zero means the cause is settled. To hold the version back, add to /etc/pacman.conf: IgnorePkg = linux-cachyos linux-cachyos-headers linux-cachyos-nvidia-open", "Sıfırsa sebep kesinleşti. Sürümü sabitlemek için /etc/pacman.conf içine: IgnorePkg = linux-cachyos linux-cachyos-headers linux-cachyos-nvidia-open"},
		"adv.kernel-downgrade.retry":   {"Remove the hold and retry when the next kernel release lands", "Sonraki çekirdek sürümünde tutmayı kaldır ve tekrar dene"},
		"adv.kernel-downgrade.gain":    {"The drops stop completely, without touching the cable", "Kopmalar tamamen biter; kabloya dokunmadan"},
		"adv.kernel-downgrade.risk":    {"An older kernel misses newer hardware support and security fixes - this is a stopgap until the regression is fixed upstream", "Eski çekirdek yeni donanım desteği ve güvenlik yamalarından geri kalır - yukarı akışta düzelene kadar geçici önlem"},
		"adv.kernel-report.title":      {"Report the regression upstream", "Regresyonu yukarı akışa bildir"},
		"adv.kernel-report.why":        {"A driver regression that only you work around stays broken for everyone else. You already have the evidence: %s.", "Yalnızca senin etrafından dolaştığın bir sürücü regresyonu herkes için bozuk kalır. Kanıt zaten elinde: %s."},
		"adv.kernel-report.s1":         {"Collect the evidence: lspci -nn | grep -i ethernet, uname -r, the journalctl link lines", "Kanıtı topla: lspci -nn | grep -i ethernet, uname -r, journalctl link satırları"},
		"adv.kernel-report.s2":         {"Open an issue on the distribution's tracker, mention the working and the broken kernel version", "Dağıtımın izleyicisinde kayıt aç, çalışan ve bozulan çekirdek sürümünü belirt"},
		"adv.kernel-report.s3":         {"Attach a nabiz report: nabiz deep --md report.md", "Rapor ekle: nabiz deep --md rapor.md"},
		"adv.kernel-report.gain":       {"It gets fixed upstream and you can go back to current kernels", "Yukarı akışta düzelir ve güncel çekirdeğe dönebilirsin"},

		"adv.kernel-bisect.title": {"Boot the LTS kernel and compare - this settles software versus hardware", "LTS çekirdekle aç ve karşılaştır - yazılım mı donanım mı sorusunu bu kapatır"},
		"adv.kernel-bisect.why":   {"The drops started recently (%s). If the cable has been the same for years and the problem appeared suddenly, one test rules software in or out: boot a different kernel.", "Düşmeler yakın zamanda başlamış (%s). Kablo yıllardır aynıysa ve sorun birden ortaya çıktıysa, tek bir testle yazılım ihtimalini eleyebilirsin: farklı bir çekirdekle aç."},
		"adv.kernel-bisect.s1":    {"Pick the LTS kernel from the boot menu (already installed)", "Açılış menüsünden LTS çekirdeğini seç (kurulu)"},
		"adv.kernel-bisect.s2":    {"Use the machine normally for at least an hour", "En az bir saat normal kullan"},
		"adv.kernel-bisect.s3":    {"Zero or near-zero means the fault is in the kernel; the same number means hardware", "Sıfır ya da çok düşükse sorun çekirdekte; aynıysa donanımda"},
		"adv.kernel-bisect.gain":  {"One test tells you which layer the problem lives in", "Tek denemede sorunun hangi katmanda olduğu kesinleşir"},
		"adv.kernel-bisect.risk":  {"None; the LTS kernel is already installed and you can switch back at any time", "Yok; LTS çekirdek zaten kurulu, istediğin zaman geri dönersin"},

		// --- physical --------------------------------------------------------
		"adv.cable-flap.title":     {"Replace the ethernet cable", "Ethernet kablosunu değiştir"},
		"adv.cable-flap.why":       {"%s dropped %d times", "%s arayüzü %d kez düştü"},
		"adv.cable-flap.span":      {" (over %.0f min, one every %.1f min, %.0f s offline in total = %.1f%% of the time)", " (%.0f dakikada, ortalama %.1f dk arayla, toplam %.0f sn kopuk = zamanın %%%.1f'i)"},
		"adv.cable-flap.downshift": {" The kernel also printed \"Downshift occurred from negotiated speed %s, check cabling!\" %d times: gigabit negotiation is not holding. 1000BASE-T uses all four pairs while 100BASE-TX uses two - the classic signature of a broken pair.", " Çekirdek ayrıca %d kez \"Downshift occurred from negotiated speed %s, check cabling!\" yazdı: gigabit anlaşması tutmuyor. 1000BASE-T dört çiftin dördünü, 100BASE-TX ikisini kullanır - klasik kopuk çift imzası."},
		"adv.cable-flap.s1":        {"Swap in a known-good Cat5e/Cat6 cable with undamaged connectors", "Sağlam bir Cat5e/Cat6 kabloyla değiştir (uçları ezik olmayan)"},
		"adv.cable-flap.s2":        {"Try a different LAN port on the modem", "Modemde başka bir LAN portu dene"},
		"adv.cable-flap.s3":        {"Remove any switch or extension in between and connect directly to the modem", "Araya switch/uzatma varsa çıkar, doğrudan modeme bağlan"},
		"adv.cable-flap.s4":        {"If the new cable shows 'Link is Up - 1000Mbps/Full' and the Downshift line is gone, it was the cable", "Yeni kabloda 'Link is Up - 1000Mbps/Full' görüp 'Downshift' satırı kaybolduysa sorun kablodaydı"},
		"adv.cable-flap.gain":      {"The drops stop and the speed goes from 100 Mbit to gigabit; no software setting substitutes for this", "Kopmalar biter, hız 100 Mbit'ten gigabite çıkar; hiçbir yazılım ayarı bunun yerine geçmez"},

		"adv.modem-port.title": {"Change the modem end of the cable too", "Kablonun modem tarafını da değiştir"},
		"adv.modem-port.why":   {"A cable has two ends. Only the PC side has been re-seated so far; the modem port and the connector at that end are still untested.", "Kablonun iki ucu var. Şimdiye kadar yalnızca PC tarafı söküldü; modem portu ve o uçtaki konnektör henüz test edilmedi."},
		"adv.modem-port.s1":    {"Move the cable to a different LAN port on the modem", "Kabloyu modemde başka bir LAN portuna tak"},
		"adv.modem-port.s2":    {"Re-seat the RJ45 at the modem end; a broken latch means the plug is not holding", "Modem tarafındaki RJ45 ucunu söküp tak; tırnağı kırıksa fiş yerinde durmuyordur"},
		"adv.modem-port.s3":    {"If the modem rebooted or updated its firmware recently, the negotiation behaviour on that side may have changed", "Modem yakın zamanda yeniden başladıysa veya firmware güncellediyse karşı taraftaki anlaşma davranışı değişmiş olabilir"},
		"adv.modem-port.gain":  {"You test the far end without needing a new cable", "Kablo değiştirmeden karşı ucu test etmiş olursun"},

		"adv.pin-100full.title": {"Stop advertising gigabit to break the failed-negotiation loop", "Gigabit denemesini kapat - başarısız anlaşma döngüsünü kes"},
		"adv.pin-100full.why":   {"The link already runs at 100 Mbit/s, but the PHY tries gigabit first every time, fails, and downshifts (%d times). Advertising only 100baseT/Full means the loop never starts.", "Bağlantı zaten 100 Mbit/s'te çalışıyor ama PHY her seferinde önce gigabit deniyor, başarısız oluyor ve downshift ediyor (%d kez). Yalnızca 100baseT/Full reklam edersen bu döngü hiç başlamaz."},
		"adv.pin-100full.s1":    {"Watch for 20-30 minutes", "20-30 dakika izle"},
		"adv.pin-100full.s2":    {"If the drops stop, the problem was in gigabit negotiation (a cable pair or the modem port)", "Düşmeler kesilirse sorun gigabit anlaşmasındaydı (kablo çifti ya da modem portu)"},
		"adv.pin-100full.gain":  {"No throughput lost (it was already 100 Mbit) and the negotiation-driven outages stop", "Hız kaybı yok (zaten 100 Mbit) ama anlaşma kaynaklı kopmalar biter"},
		"adv.pin-100full.risk":  {"The link renegotiates once when the command runs: a few seconds of downtime", "Komut çalıştığı an link bir kez yeniden anlaşır: birkaç saniye kopar"},

		"adv.eee-off.title": {"Turn EEE off - the free test, before buying a cable", "EEE'yi kapat - kablo almadan önceki bedava test"},
		"adv.eee-off.why":   {"Energy Efficient Ethernet is enabled and active. On the r8169 driver it is a known cause of exactly this: the PHY powers the link down between packets and sometimes fails to bring it back. Do this before replacing the cable - it is one reversible command and the answer arrives in half an hour, while a cable means a trip to the shop. If the drops continue, the cable is still the next suspect.", "Energy Efficient Ethernet etkin ve aktif. r8169 sürücüsünde tam olarak bunun bilinen bir nedenidir: PHY paketler arasında linki uyutur ve bazen geri getiremez. Bunu kabloyu değiştirmeden önce yap - tek komut, geri alınabilir ve cevap yarım saatte gelir; kablo ise dükkâna gitmek demek. Düşmeler sürerse sıradaki şüpheli yine kablodur."},
		"adv.eee-off.s1":    {"Watch for 15-20 minutes", "15-20 dakika izle"},
		"adv.eee-off.s2":    {"If the drops stop, make it survive reboots and relinks with a NetworkManager dispatcher - a boot-time unit alone is not enough, because the setting lives in the PHY and a renegotiation can clear it", "Düşmeler durursa, yeniden başlatmayı ve her yeni anlaşmayı atlatması için NetworkManager dispatcher ekle - yalnızca açılışta çalışan bir birim yetmez, çünkü ayar PHY'da durur ve yeniden anlaşma onu silebilir"},
		"adv.eee-off.gain":  {"EEE-induced flaps stop", "EEE kaynaklı flap'ler biter"},
		"adv.eee-off.risk":  {"A negligible increase in power draw", "İhmal edilebilir düzeyde ek güç tüketimi"},

		"adv.link-speed.title": {"The link negotiated %d Mbit/s - worth investigating if you expect gigabit", "Bağlantı %d Mbit/s'te anlaşmış - gigabit bekliyorsan araştır"},
		"adv.link-speed.why":   {"If one pair in the cable is broken or the cable is Cat5, the card falls back to 100 Mbit. That caps you regardless of the line speed you pay for.", "Kablonun bir çifti kopuksa veya kablo Cat5 ise kart 100 Mbit'e düşer. Bu, hattın hızından bağımsız olarak tavan koyar."},
		"adv.link-speed.s1":    {"Check the Speed and Advertised link modes lines", "Speed ve Advertised link modes satırlarına bak"},
		"adv.link-speed.s2":    {"Swap the cable and look again", "Kabloyu değiştirip tekrar bak"},
		"adv.link-speed.s3":    {"If the modem itself is not gigabit, 100 Mbit is the ceiling anyway", "Modem gigabit değilse tavan zaten 100 Mbit'tir"},
		"adv.link-speed.gain":  {"If the line really is faster than 100 Mbit, download speed multiplies", "Hat gerçekten 100 Mbit'ten hızlıysa indirme hızı katlanır"},

		"adv.duplex.title": {"Half duplex detected", "Half duplex tespit edildi"},
		"adv.duplex.why":   {"Collisions are unavoidable in half duplex; loss and latency become permanent.", "Half duplex'te çarpışma kaçınılmazdır; kayıp ve gecikme kalıcı olur."},
		"adv.duplex.s1":    {"If that does not fix it, the cable or the switch port is faulty", "Düzelmezse kablo veya switch portu arızalıdır"},
		"adv.duplex.gain":  {"A significant drop in loss", "Kayıp oranında ciddi düşüş"},

		"adv.crc.title": {"CRC errors - the cable is physically damaged", "CRC hataları var - kablo fiziksel olarak bozuk"},
		"adv.crc.why":   {"rx_crc_errors = %d. A CRC error means the packet was corrupted on the wire; no software setting repairs that.", "rx_crc_errors = %d. CRC hatası, paketin kablo üzerinde bozulduğu anlamına gelir; yazılımla düzeltilemez."},
		"adv.crc.s1":    {"Replace the cable", "Kabloyu değiştir"},
		"adv.crc.s2":    {"Check the connectors", "Konnektör uçlarını kontrol et"},
		"adv.crc.s3":    {"Keep the cable away from power leads", "Kabloyu güç kablolarından uzaklaştır"},
		"adv.crc.gain":  {"Retransmissions and stalls stop", "Yeniden gönderimler ve donmalar biter"},

		"adv.wifi.title": {"Wi-Fi signal is weak (%.0f dBm)", "Wi-Fi sinyali zayıf (%.0f dBm)"},
		"adv.wifi.why":   {"Below -70 dBm the retransmission rate climbs quickly; jitter and loss follow.", "-70 dBm altında yeniden gönderim oranı hızla artar; jitter ve kayıp bunun sonucudur."},
		"adv.wifi.s1":    {"Move to the 5 GHz band (2.4 GHz is crowded)", "5 GHz bandına geç (2.4 GHz kalabalıktır)"},
		"adv.wifi.s2":    {"Move closer to the access point or change its orientation", "Erişim noktasına yaklaş veya yönünü değiştir"},
		"adv.wifi.s3":    {"Switch to cable if you can: the difference shows immediately in games and calls", "Mümkünse kabloya geç: oyunda ve görüşmede fark hemen görülür"},
		"adv.wifi.gain":  {"Jitter and loss drop noticeably", "Jitter ve kayıp belirgin düşer"},

		"adv.wifi-channel.title": {"Check how crowded your Wi-Fi channel is", "Wi-Fi kanalının ne kadar kalabalık olduğuna bak"},
		"adv.wifi-channel.why":   {"Jitter measured %.1f ms on Wi-Fi. Overlapping networks on the same channel cause exactly this: the radio waits, and the wait shows up as jitter rather than loss.", "Wi-Fi üzerinde jitter %.1f ms ölçüldü. Aynı kanaldaki komşu ağlar tam olarak bunu yapar: telsiz bekler ve bu bekleme kayıp yerine jitter olarak görünür."},
		"adv.wifi-channel.s1":    {"Pick the least crowded channel in the router's interface (1/6/11 on 2.4 GHz)", "Router arayüzünden en boş kanalı seç (2.4 GHz'de 1/6/11)"},
		"adv.wifi-channel.gain":  {"Lower jitter without any hardware change", "Donanım değişmeden jitter düşer"},

		"adv.lan-loss.title": {"The loss starts inside your home", "Kayıp ev içinde başlıyor"},
		"adv.lan-loss.why":   {"%.1f%% loss was measured on the way to the modem. That is not the ISP's problem, it is the local connection.", "Modeme giden yolda %%%.1f kayıp ölçüldü. Bu ISS'nin değil, ev içi bağlantının sorunudur."},
		"adv.lan-loss.s1":    {"Change the cable or the Wi-Fi connection and repeat the test", "Kablo/Wi-Fi bağlantısını değiştirip testi tekrarla"},
		"adv.lan-loss.s2":    {"If a switch sits in between, connect directly to the modem and compare", "Araya switch varsa doğrudan modeme bağlanıp karşılaştır"},
		"adv.lan-loss.gain":  {"It narrows the fault down; the first thing to do before calling the ISP", "Sorunun kaynağını daraltır; ISS'yi aramadan önce yapılacak ilk şey"},

		"adv.switch-bypass.title": {"Take the intermediate switch out of the path", "Aradaki switch'i devreden çıkar"},
		"adv.switch-bypass.why":   {"An unmanaged switch with a failing port or a cheap power supply produces exactly this pattern: brief total outages with no ISP-side trace.", "Portu bozulmuş ya da ucuz güç kaynaklı yönetilmeyen bir switch tam olarak bu deseni üretir: ISS tarafında izi olmayan kısa tam kopmalar."},
		"adv.switch-bypass.s1":    {"Connect the machine straight to the modem for an hour", "Makineyi bir saatliğine doğrudan modeme bağla"},
		"adv.switch-bypass.s2":    {"If the drops stop, the switch is the culprit", "Düşmeler kesilirse suçlu switch'tir"},
		"adv.switch-bypass.gain":  {"Rules out a whole device without buying anything", "Hiçbir şey almadan bir cihazı tamamen eler"},

		// --- queue / bufferbloat -------------------------------------------
		"apply.measuring":       {"Putting the line back under load to see whether it actually helped...", "Gerçekten işe yarayıp yaramadığını görmek için hat tekrar yüke sokuluyor..."},
		"apply.measurefailed":   {"The measurement did not complete, so the change is left in place - run nabiz load yourself to check it", "Ölçüm tamamlanamadı, bu yüzden değişiklik yerinde bırakıldı - kontrol için nabiz load çalıştırın"},
		"apply.effect":          {"%s: %.1f -> %.1f", "%s: %.1f -> %.1f"},
		"apply.effectworse":     {"That is worse than before the change, so it is being undone", "Bu, değişiklik öncesinden daha kötü; geri alınıyor"},
		"adv.sqm.title":         {"Shape both directions with cake to cut bufferbloat", "cake ile her iki yönü şekillendirip bufferbloat'ı kes"},
		"adv.sqm.why":           {"Latency rises by +%.0f ms under load (grade %s): +%.0f ms while downloading, +%.0f ms while uploading. A queue is swelling somewhere between here and the internet, and games and calls break even when the speed reads fine. Upload is shaped on this interface, because that queue is ours. Download is shaped through an IFB device, because its queue is inside the modem, one hop upstream, where no local qdisc can reach it - shaping ingress makes the far-end senders slow down instead, and the modem drains.", "Yük altında gecikme +%.0f ms artıyor (not %s): indirirken +%.0f ms, yüklerken +%.0f ms. Buradan internete kadar bir yerde kuyruk şişiyor; hız yeterli görünse de oyun ve görüşme bozulur. Yükleme bu arayüzde şekillendirilir, çünkü o kuyruk bizim. İndirme ise IFB aygıtı üzerinden şekillendirilir, çünkü onun kuyruğu bir üst atlamada, modemin içinde ve hiçbir yerel qdisc oraya erişemez - ingress'i şekillendirmek karşı taraftaki göndericileri yavaşlatır, modem de boşalır."},
		"adv.sqm.s1":            {"Rates come from what this line actually delivered: download ~%d Mbit, upload ~%d Mbit. Download keeps the wider margin - it is the harder half to control.", "Hızlar bu hattın gerçekten verdiğinden türetildi: indirme ~%d Mbit, yükleme ~%d Mbit. İndirmede pay daha geniş tutuldu - kontrolü zor olan yarı orası."},
		"adv.sqm.s2":            {"If the grade is still not A, lower the download rate in steps of about 5 Mbit and measure again; the point where latency drops sharply is the one to keep", "Not hâlâ A değilse indirme hızını ~5 Mbit'lik adımlarla düşürüp tekrar ölç; gecikmenin sert biçimde düştüğü nokta doğru noktadır"},
		"adv.sqm.s3":            {"These commands are lost at the next relink or reboot - run nabiz apply --only sqm to have it installed as a NetworkManager dispatcher instead. Better still, if the router itself supports SQM (OpenWrt: luci-app-sqm with cake), do it there and undo this: shaping at the bottleneck beats shaping behind it.", "Bu komutlar bir sonraki yeniden bağlanmada veya açılışta kaybolur - kalıcı olması için nabiz apply --only sqm çalıştır, NetworkManager dispatcher olarak kurulsun. Daha iyisi: router SQM destekliyorsa (OpenWrt: cake ile luci-app-sqm) orada yap ve buradakini geri al; darboğazın kendisinde şekillendirmek, arkasında şekillendirmekten iyidir."},
		"adv.sqm-persist.title": {"Make the shaping survive a relink", "Şekillendirmeyi yeniden bağlanmaya dayanıklı yap"},
		"adv.sqm-persist.why":   {"cake is in force right now, but a tc command lives only until the link goes down. Every renegotiation, suspend or reboot drops it, and the bufferbloat returns without anything appearing to have changed.", "cake şu anda etkin, ama bir tc komutu yalnızca link düşene kadar yaşar. Her yeniden anlaşma, uyku veya yeniden başlatma onu siler; hiçbir şey değişmemiş gibi görünürken bufferbloat geri gelir."},
		"adv.sqm-persist.s1":    {"Keep the rates that are working now", "Şu an işe yarayan hızları koru"},
		"adv.sqm-persist.s2":    {"After the next reboot, confirm with: tc qdisc show dev %s", "Sonraki açılıştan sonra şununla doğrula: tc qdisc show dev %s"},
		"adv.sqm-persist.gain":  {"The shaping is reapplied automatically on every link-up", "Şekillendirme her link-up olayında otomatik olarak yeniden uygulanır"},
		"adv.sqm.gain":          {"Latency under load usually falls into the 10-30 ms band and the bufferbloat grade becomes A", "Yük altında gecikme genelde 10-30 ms bandına iner ve bufferbloat notu A olur"},
		"adv.sqm.risk":          {"Shaping trades peak throughput for latency - expect to give up roughly a tenth of upload and up to a quarter of download. If the bottleneck is your own LAN link rather than the ISP line, that cost is larger and a faster link removes the need for shaping entirely.", "Şekillendirme tepe hızı karşılığında gecikme satın alır - yüklemenin kabaca onda birinden, indirmenin dörtte birine kadarından vazgeçmeyi bekle. Darboğaz ISS hattı değil de kendi LAN bağlantınsa bu bedel daha büyüktür; daha hızlı bir bağlantı şekillendirme ihtiyacını tamamen ortadan kaldırır."},

		"adv.default-qdisc.title": {"Set the default queue discipline to fq_codel", "Varsayılan kuyruk disiplinini fq_codel yap"},
		"adv.default-qdisc.why":   {"It is currently %s. Disciplines without queue management inflate local latency.", "Şu an %s. Kuyruk yönetimi olmayan disiplinler yerel gecikmeyi şişirir."},
		"adv.default-qdisc.gain":  {"Local queueing latency drops", "Yerel kuyruk gecikmesi düşer"},

		"adv.cake-gaming.title": {"Shape the upload half with cake (no IFB here, so download cannot be shaped)", "Yükleme yarısını cake ile şekillendir (burada IFB yok, indirme şekillendirilemez)"},
		"adv.cake-gaming.why":   {"Upload latency rises by +%.0f ms under load. With diffserv marking, a bulk upload stops delaying game and call packets.", "Yükleme sırasında gecikme +%.0f ms artıyor. diffserv işaretlemesiyle büyük bir yükleme, oyun ve görüşme paketlerini geciktirmez."},
		"adv.cake-gaming.s1":    {"Measure again while uploading something large", "Büyük bir yükleme sürerken tekrar ölç"},
		"adv.cake-gaming.gain":  {"Ping stays flat even while an upload is saturating the link", "Yükleme hattı doldururken bile ping sabit kalır"},
		"adv.cake-gaming.risk":  {"Only helps if the traffic is actually marked; many applications do not mark", "Yalnızca trafik gerçekten işaretliyse işe yarar; birçok uygulama işaretlemez"},

		// --- kernel tunables ------------------------------------------------
		"adv.bbr.title": {"Try the bbr congestion control on a lossy link", "Kayıplı hatta bbr tıkanıklık kontrolünü dene"},
		"adv.bbr.why":   {"The retransmission rate is %.2f%%. cubic reads loss as congestion and cuts the rate unnecessarily; bbr decides from measured latency and bandwidth instead.", "Yeniden gönderim oranı %%%.2f. cubic kaybı tıkanıklık sayar ve hızı gereksiz kısar; bbr gecikme/bant ölçümüne göre davranır."},
		"adv.bbr.s1":    {"Compare before and after", "Önce/sonra karşılaştır"},
		"adv.bbr.s2":    {"To persist it, add the setting to /etc/sysctl.d/99-nabiz.conf", "Kalıcı yapmak için ayarı /etc/sysctl.d/99-nabiz.conf içine ekle"},
		"adv.bbr.gain":  {"On lossy links, upload speed and stability improve", "Kayıplı hatlarda yükleme hızı ve kararlılığı artar"},
		"adv.bbr.risk":  {"bbr can fill queues on some paths; do not make it permanent without measuring", "bbr bazı yollarda kuyruk doldurabilir; ölçmeden kalıcı yapma"},

		"adv.mtu-probe.title": {"Enable MTU probing for the PMTU black hole", "PMTU kara deliği için MTU probing aç"},
		"adv.mtu-probe.why":   {"The interface MTU is %d but only %d gets through the path. Large packets vanish silently: the page starts loading and then hangs.", "Arayüz MTU'su %d ama yoldan yalnızca %d geçiyor. Büyük paketler sessizce düşüyor: sayfa açılmaya başlayıp donuyor."},
		"adv.mtu-probe.s1":    {"If it persists, lower the interface MTU", "Sürerse arayüz MTU'sunu düşür"},
		"adv.mtu-probe.gain":  {"Hanging pages and half-finished downloads recover", "Takılan sayfalar ve yarım kalan indirmeler düzelir"},

		"adv.ssaio.title": {"Disable slow start after idle", "Boşta kaldıktan sonra yavaş başlangıcı kapat"},
		"adv.ssaio.why":   {"Connections that go quiet for a while (video, games) ramp up from scratch after every pause, which feels like stuttering.", "Uzun süre sessiz kalan bağlantılar (video, oyun) her duraklamadan sonra sıfırdan hızlanır; bu, akışta takılma olarak hissedilir."},
		"adv.ssaio.gain":  {"Streaming and game traffic recovers faster after a pause", "Akış ve oyun trafiğinde duraklama sonrası toparlanma hızlanır"},

		"adv.notsent-lowat.title": {"Cap the socket send queue with tcp_notsent_lowat", "tcp_notsent_lowat ile soket gönderme kuyruğunu sınırla"},
		"adv.notsent-lowat.why":   {"Latency rises by +%.0f ms while uploading. A large part of that queue is on this machine, not in the modem: applications hand the kernel more data than the link can carry.", "Yükleme sırasında gecikme +%.0f ms artıyor. Bu kuyruğun önemli kısmı modemde değil bu makinede: uygulamalar çekirdeğe hattın taşıyabileceğinden fazla veri veriyor."},
		"adv.notsent-lowat.s1":    {"Measure the upload again and watch p95", "Yüklemeyi tekrar ölç ve p95'e bak"},
		"adv.notsent-lowat.gain":  {"Lower local queueing latency during uploads with no throughput cost", "Yükleme sırasında yerel kuyruk gecikmesi düşer, hız kaybı olmaz"},

		"adv.conntrack.title": {"Raise the conntrack table", "conntrack tablosunu büyüt"},
		"adv.conntrack.why":   {"The table is %.0f%% full. Once it fills, new connections are dropped with no error anywhere - it looks exactly like an ISP outage.", "Tablo %%%.0f dolu. Dolduğunda yeni bağlantılar hiçbir yerde hata vermeden düşer - tam olarak ISS kesintisi gibi görünür."},
		"adv.conntrack.gain":  {"Connections stop failing during heavy usage", "Yoğun kullanımda bağlantı kurulamama sorunu biter"},

		"adv.nic-offload.title": {"Test with NIC offloads disabled", "NIC offload'larını kapatarak test et"},
		"adv.nic-offload.why":   {"Retransmission is %.2f%% while the ping to the modem is clean. Some Realtek chips corrupt or reorder segments with GRO/TSO enabled, which the kernel then counts as loss.", "Modeme ping temizken yeniden gönderim %%%.2f. Bazı Realtek yongaları GRO/TSO açıkken segmentleri bozar veya sırasını değiştirir; çekirdek de bunu kayıp sayar."},
		"adv.nic-offload.s1":    {"Measure again; if the retransmission rate drops, keep it off", "Tekrar ölç; yeniden gönderim düşerse kapalı bırak"},
		"adv.nic-offload.gain":  {"Fewer retransmissions and fewer stalls", "Daha az yeniden gönderim ve donma"},
		"adv.nic-offload.risk":  {"Slightly higher CPU usage at high throughput", "Yüksek hızda biraz daha fazla CPU kullanımı"},

		"adv.aspm.title": {"Disable PCIe power management for the network card", "Ağ kartı için PCIe güç yönetimini kapat"},
		"adv.aspm.why":   {"The link keeps dropping and EEE is not the cause. ASPM (PCIe power saving) is the other well-known trigger on Realtek cards: the card is put to sleep and wakes up badly.", "Bağlantı düşmeye devam ediyor ve sebep EEE değil. Realtek kartlarda diğer bilinen tetikleyici ASPM (PCIe güç tasarrufu): kart uyutuluyor ve kötü uyanıyor."},
		"adv.aspm.s1":    {"Add pcie_aspm=off to the kernel command line and reboot", "Çekirdek komut satırına pcie_aspm=off ekleyip yeniden başlat"},
		"adv.aspm.s2":    {"Count the drops again after an hour", "Bir saat sonra düşmeleri tekrar say"},
		"adv.aspm.gain":  {"ASPM-induced link drops stop", "ASPM kaynaklı link düşmeleri biter"},
		"adv.aspm.risk":  {"A small increase in idle power draw across the whole system", "Sistem genelinde boşta güç tüketimi biraz artar"},

		"adv.aspm-force.title":  {"Hand PCIe power management to the kernel so the driver can switch it off", "PCIe güç yönetimini çekirdeğe devret ki sürücü kapatabilsin"},
		"adv.aspm-force.why":    {"The kernel logged \"%s %s: can't disable ASPM; OS doesn't have ASPM control\". The driver disables ASPM on this chip deliberately, but the firmware never handed ASPM over through ACPI _OSC, so the request was dropped and the card still runs with PCIe power saving on. This is measured, not suspected. pcie_aspm=off does not help - it tells the kernel to leave ASPM alone, keeping the firmware setting; pcie_aspm=force makes the kernel take control so the driver's own disable succeeds.", "Çekirdek şunu yazdı: \"%s %s: can't disable ASPM; OS doesn't have ASPM control\". Sürücü bu yongada ASPM'i bilerek kapatır ama firmware ACPI _OSC üzerinden ASPM'i hiç devretmediği için istek düştü ve kart hâlâ PCIe güç tasarrufuyla çalışıyor. Bu tahmin değil, ölçüm. pcie_aspm=off işe yaramaz - çekirdeğe ASPM'e dokunmamasını söyler, firmware ayarı kalır; pcie_aspm=force ise kontrolü çekirdeğe verir ve sürücünün kapatma isteği geçer."},
		"adv.aspm-force.s1":     {"Back up the bootloader config first: sudo cp /etc/default/grub /etc/default/grub.bak", "Önce önyükleyici ayarını yedekle: sudo cp /etc/default/grub /etc/default/grub.bak"},
		"adv.aspm-force.s2":     {"Reboot, then confirm the message is gone and count the link drops again after an hour", "Yeniden başlat, sonra mesajın kaybolduğunu doğrula ve bir saat sonra link düşmelerini tekrar say"},
		"adv.aspm-force.gain":   {"The driver's own workaround finally applies, removing the most likely cause of the drops", "Sürücünün kendi çözümü nihayet uygulanır, düşmelerin en olası nedeni ortadan kalkar"},
		"adv.aspm-force.risk":   {"Slightly higher idle power draw, and the kernel manages ASPM on every PCIe device rather than only this one", "Boşta güç tüketimi biraz artar ve çekirdek ASPM'i yalnızca bu kartta değil tüm PCIe cihazlarında yönetir"},
		"adv.aspm-force.revert": {"Restore /etc/default/grub from the backup, run sudo grub-mkconfig -o /boot/grub/grub.cfg and reboot", "/etc/default/grub dosyasını yedekten geri al, sudo grub-mkconfig -o /boot/grub/grub.cfg çalıştır ve yeniden başlat"},

		"adv.journal-persist.title": {"Make the system log survive reboots", "Sistem kaydının yeniden başlatmayı atlatmasını sağla"},
		"adv.journal-persist.why":   {"Only %d boots can be read right now, because journald keeps its log in memory and throws it away on shutdown. That makes the one question worth asking - does this kernel drop the link more than the previous one - unanswerable, and worse than unanswerable: a kernel nobody logged scores zero drops in zero observed minutes and looks perfect. Turn this on first, then every experiment below can be judged across a reboot.", "Şu anda yalnızca %d açılış okunabiliyor, çünkü journald kaydı bellekte tutuyor ve kapanışta atıyor. Bu, sorulmaya değer tek soruyu - bu çekirdek bağlantıyı öncekinden daha mı çok düşürüyor - cevaplanamaz kılıyor; dahası yanıltıyor: kaydı tutulmamış bir çekirdek sıfır gözlenen dakikada sıfır düşmeyle kusursuz görünür. Önce bunu aç, sonra aşağıdaki her deney yeniden başlatma boyunca değerlendirilebilir."},
		"adv.journal-persist.s1":    {"Reboot at least once on each kernel you want to compare, then run nabiz again", "Karşılaştırmak istediğin her çekirdekte en az bir kez yeniden başlat, sonra nabiz'i tekrar çalıştır"},
		"adv.journal-persist.gain":  {"Kernels become comparable, so a real regression can be separated from a hardware fault", "Çekirdekler karşılaştırılabilir olur; gerçek bir regresyon donanım arızasından ayrılabilir"},
		"adv.journal-persist.risk":  {"The log takes disk space, capped by journald at a tenth of the filesystem by default", "Kayıt disk yeri kaplar; journald varsayılan olarak dosya sisteminin onda biri ile sınırlar"},

		// --- bpftune -----------------------------------------------------------
		"adv.bpftune-buffers.title": {"Pin the receive buffer bpftune grew to a sane ceiling", "bpftune'un büyüttüğü alım tamponunu makul bir tavana sabitle"},
		"adv.bpftune-buffers.why":   {"The tcp_rmem ceiling is %.0f MB while this line's bandwidth-delay product (%d Mbit × %.0f ms) is only %.0f KB. The difference does not become throughput; it comes back as queueing and latency under congestion.", "tcp_rmem tavanı %.0f MB; bu hattın bant-gecikme çarpımı (%d Mbit × %.0f ms) yalnızca %.0f KB. Aradaki fark hıza dönüşmez, tıkanıklık anında kuyruk ve gecikme olarak geri döner."},
		"adv.bpftune-buffers.s1":    {"Measure the result", "Sonucu ölç"},
		"adv.bpftune-buffers.s2":    {"If bpftune grows it again, disable the buffer tuner or roll its changes back:", "bpftune tekrar büyütürse tampon ayarlayıcısını kapat ya da değişikliklerini geri al:"},
		"adv.bpftune-buffers.gain":  {"Lower latency and p95 under load, with no expected loss of throughput", "Yük altında gecikme ve p95 düşer; hız kaybı beklenmez"},
		"adv.bpftune-buffers.risk":  {"On long-distance, high-speed transfers (overseas servers, 1 Gbit+) a lower ceiling can cost throughput", "Uzun mesafeli, yüksek hızlı transferlerde (yurt dışı sunucu, 1 Gbit+) tavanı düşürmek hızı kısabilir"},

		"adv.bpftune-retrans.title": {"Check whether bpftune is causing the retransmission", "Yeniden gönderimin sebebi bpftune mu, ölç"},
		"adv.bpftune-retrans.why":   {"%.1f%% of the segments sent during the upload test had to be sent again, while the kernel-wide figure is only %.1f%% - the loss is concentrated exactly where bpftune is choosing a congestion control algorithm per socket. It puts dctcp in the allowed list, and dctcp needs end-to-end ECN; across the internet it behaves like reno and gives up throughput for nothing. Measure it before deciding: the A/B run below turns bpftune off and on around the same test.", "Yükleme testi sırasında gönderilen segmentlerin %%%.1f'i tekrar gönderilmek zorunda kaldı; çekirdek geneli oran ise yalnızca %%%.1f - kayıp tam olarak bpftune'un soket başına tıkanıklık algoritması seçtiği yerde yoğunlaşıyor. İzinli listeye dctcp ekliyor, dctcp ise uçtan uca ECN ister; internet üzerinde reno gibi davranır ve karşılığında hiçbir şey vermeden hızdan yer. Karar vermeden önce ölç: aşağıdaki A/B testi aynı ölçümü bpftune kapalı ve açıkken çalıştırır."},
		"adv.bpftune-retrans.s1":    {"If the run with bpftune off retransmits less at the same throughput, keep it off", "bpftune kapalı çalışma aynı hızda daha az yeniden gönderim yapıyorsa kapalı bırak"},
		"adv.bpftune-retrans.s2":    {"If it made no difference, turn it back on - it earns its place on some links", "Fark etmediyse geri aç - bazı hatlarda işe yarar"},
		"adv.bpftune-retrans.gain":  {"Retransmission falls to a fraction of a percent and the throughput stays where it was", "Yeniden gönderim yüzdenin küçük bir kesrine iner ve hız olduğu yerde kalır"},
		"adv.bpftune-retrans.risk":  {"bpftune also tunes buffers and backlogs; turning it off gives those back to the kernel defaults, which is usually what you want on a well-shaped link", "bpftune tamponları ve kuyrukları da ayarlar; kapatmak onları çekirdek varsayılanlarına bırakır, ki düzgün şekillendirilmiş bir hatta genelde istenen budur"},
		"adv.bpftune-cc.title":      {"Verify bpftune's per-connection algorithm choice", "bpftune'un bağlantı başına algoritma seçimini doğrula"},
		"adv.bpftune-cc.why":        {"Different congestion control algorithms are in use per socket (%s). That is usually good, but dctcp behaves like reno over the internet without end-to-end ECN.", "Soket bazında farklı tıkanıklık algoritmaları kullanılıyor (%s). Bu genelde iyidir ama dctcp internet üzerinde ECN olmadan reno gibi davranır."},
		"adv.bpftune-cc.s1":         {"Run the same suite with bpftune on and off", "Aynı testi bpftune açık ve kapalıyken çalıştır"},
		"adv.bpftune-cc.s2":         {"If the difference does not show in p95 latency and retransmission, turning bpftune off is also an option", "Fark p95 gecikmede ve yeniden gönderimde görünmüyorsa bpftune'u kapatmak da bir seçenek"},
		"adv.bpftune-cc.gain":       {"A decision based on measurement instead of guesswork", "Tahmin yerine ölçüye dayalı karar"},

		"adv.bpftune-vs-physical.title": {"Fix the physical loss first, then look at bpftune", "Önce fiziksel kaybı çöz, sonra bpftune'a bak"},
		"adv.bpftune-vs-physical.why":   {"Retransmission is %.2f%%. bpftune reads that as 'the buffer is too small' and grows it; but if the loss comes from the cable, a bigger buffer hides the problem instead of fixing it.", "Yeniden gönderim %%%.2f. bpftune bu tabloyu 'tampon yetmiyor' diye yorumlayıp tamponu büyütür; oysa kayıp kablodan geliyorsa büyük tampon sorunu gizler, çözmez."},
		"adv.bpftune-vs-physical.s1":    {"Do the cable/port swap first (the physical advice above)", "Önce kablo/port değişimini yap (yukarıdaki fiziksel öneriler)"},
		"adv.bpftune-vs-physical.s2":    {"Then measure again", "Sonra tekrar ölç"},
		"adv.bpftune-vs-physical.gain":  {"bpftune tunes far more accurately on a clean link", "bpftune temiz bir hat üzerinde çok daha isabetli ayar yapar"},

		"adv.bpftune-not-needed.title": {"You do not appear to need an auto-tuner like bpftune", "bpftune gibi otomatik ayarlayıcılara ihtiyacın yok görünüyor"},
		"adv.bpftune-not-needed.why":   {"Latency rises very little under load and the retransmission rate is negligible. Growing kernel buffers in this situation buys nothing measurable.", "Yük altında gecikme artışı düşük ve yeniden gönderim oranı ihmal edilebilir. Bu tabloda çekirdek tamponlarını büyütmek ölçülebilir bir kazanç getirmez."},
		"adv.bpftune-not-needed.s1":    {"Do nothing; always measure before and after changing a setting", "Bir şey yapma; ayar değiştirmeden önce hep önce/sonra ölç"},
		"adv.bpftune-not-needed.gain":  {"Avoiding needless complexity", "Gereksiz karmaşadan kaçınmak"},

		// --- dns ----------------------------------------------------------------
		"adv.dns-leak.title": {"Encrypted DNS is on but plaintext resolvers are still in reserve", "Şifreli DNS açık ama düz metin çözücüler hâlâ yedekte"},
		"adv.dns-leak.why":   {"When the encrypted resolver does not answer, systemd-resolved asks %s in plaintext. A single timeout is enough to bypass the encryption.", "Şifreli çözücü yanıt vermediğinde systemd-resolved %s adreslerine düz metin sorar. Tek bir zaman aşımı şifrelemeyi devre dışı bırakmaya yeter."},
		"adv.dns-leak.s1":    {"Leave the FallbackDNS= line empty", "FallbackDNS= satırını boş bırak"},
		"adv.dns-leak.s2":    {"Verify with nabiz dns", "nabiz dns ile doğrula"},
		"adv.dns-leak.gain":  {"DNS queries really do stay encrypted", "DNS sorguları gerçekten şifreli kalır"},

		"adv.dns-encrypt.title": {"Turn on encrypted DNS", "Şifreli DNS'i aç"},
		"adv.dns-encrypt.why":   {"Plaintext DNS is both the one thing your ISP can see and the one thing that can be modified on the path. Even when this test finds no interference, DoH/DoT gives you privacy and immunity to hijacking.", "Düz metin DNS, hem ISS'nin gördüğü hem de yolda değiştirilebilen tek bileşendir. Bu testte müdahale çıkmasa bile DoH/DoT hem gizlilik hem kaçırmaya karşı koruma sağlar."},
		"adv.dns-encrypt.s1":    {"With Unwall installed: unwallctl dns enable quad9 dnscrypt", "Unwall kuruluysa: unwallctl dns enable quad9 dnscrypt"},
		"adv.dns-encrypt.s2":    {"Or directly: systemd-resolved with DNSOverTLS=yes", "Ya da doğrudan: systemd-resolved içinde DNSOverTLS=yes"},
		"adv.dns-encrypt.gain":  {"Privacy plus immunity to DNS hijacking", "Gizlilik + DNS kaçırmaya karşı bağışıklık"},
		"adv.dns-encrypt.risk":  {"About 50-150 ms extra on the first query; it disappears once the cache is warm", "İlk sorguda ~50-150 ms ek gecikme; önbellek ısınınca kaybolur"},

		"adv.dns-cache.title": {"Run a local caching resolver", "Yerel önbellekli çözücü çalıştır"},
		"adv.dns-cache.why":   {"The fastest resolver still answers in %.0f ms. Every page load makes several lookups, and most of them repeat - a local cache turns those into microseconds.", "En hızlı çözücü bile %.0f ms'de yanıt veriyor. Her sayfa açılışı birkaç sorgu yapar ve çoğu tekrarlıdır - yerel önbellek bunları mikrosaniyeye indirir."},
		"adv.dns-cache.s1":    {"dnscrypt-proxy has a cache: set cache = true in its config", "dnscrypt-proxy'de önbellek var: yapılandırmasında cache = true yap"},
		"adv.dns-cache.s2":    {"Or use systemd-resolved's own cache (Cache=yes)", "Ya da systemd-resolved'ın kendi önbelleğini kullan (Cache=yes)"},
		"adv.dns-cache.gain":  {"Repeat lookups cost nothing; page loads feel snappier", "Tekrarlı sorgular bedavaya gelir; sayfa açılışları hızlanır"},

		"adv.dns-fastest.title": {"Fastest resolver measured: %s", "Ölçüme göre en hızlı çözücü: %s"},
		"adv.dns-fastest.why":   {"This resolver gave the lowest average latency in this run.", "Bu koşuda en düşük ortalama gecikmeyi bu çözücü verdi."},
		"adv.dns-fastest.s1":    {"Prefer it when choosing your encrypted DNS provider", "Şifreli DNS sağlayıcını seçerken bunu tercih et"},
		"adv.dns-fastest.s2":    {"Measure again at different times of day", "Farklı saatlerde tekrar ölç"},
		"adv.dns-fastest.gain":  {"A few tens of milliseconds off time-to-first-byte on page loads", "Sayfa açılışlarında ilk bayta kadar geçen sürede birkaç on milisaniye"},

		"adv.dns-hijack.title": {"Your ISP redirects non-existent domains to its own page", "ISS var olmayan alan adlarını kendi sayfasına yönlendiriyor"},
		"adv.dns-hijack.s1":    {"Switch to encrypted DNS (the advice above)", "Şifreli DNS'e geç (yukarıdaki öneri)"},
		"adv.dns-hijack.s2":    {"Change the modem's DNS setting too, otherwise the other devices stay affected", "Modemin DNS ayarını da değiştir, yoksa diğer cihazlar etkilenmeye devam eder"},
		"adv.dns-hijack.gain":  {"Mistyped addresses fail properly instead of landing on an ad page, and some applications stop misbehaving", "Yanlış yazılan adresler reklam sayfasına değil hataya düşer; bazı uygulamaların bozulması biter"},

		"adv.dns-transparent.title": {"UDP/53 is intercepted on the path - changing resolvers will not help", "UDP/53 yolda ele geçiriliyor - çözücü değiştirmek işe yaramaz"},
		"adv.dns-transparent.s1":    {"Encrypted DNS is mandatory here: use DoH (443) or DoT (853)", "Şifreli DNS zorunlu: DoH (443) veya DoT (853) kullan"},
		"adv.dns-transparent.s2":    {"Verify afterwards", "Sonra doğrula"},
		"adv.dns-transparent.gain":  {"Answers really come from the resolver you chose", "Yanıtlar gerçekten seçtiğin çözücüden gelir"},

		// --- dpi ------------------------------------------------------------------
		"adv.zapret-split.title": {"The DPI matches SNI in a single packet - a splitting strategy fits", "DPI, SNI'yi tek pakette arıyor - bölme stratejisi uygun"},
		"adv.zapret-split.why":   {"%s only opened when the ClientHello was split in two. The middlebox is not reassembling the TCP stream.", "%s yalnızca ClientHello ikiye bölündüğünde açıldı. Orta kutu TCP akışını birleştirmiyor."},
		"adv.zapret-split.s1":    {"Choose the multisplit or multidisorder strategy on the zapret2 engine in Unwall", "Unwall'da zapret2 motorunda multisplit veya multidisorder stratejisini seç"},
		"adv.zapret-split.s2":    {"Then verify", "Sonra doğrula"},
		"adv.zapret-split.gain":  {"Blocked sites open with no added latency", "Engelli siteler ek gecikme olmadan açılır"},

		"adv.zapret-strategy.title": {"The current zapret strategy is not enough for these targets", "Mevcut zapret stratejisi bu hedefler için yetmiyor"},
		"adv.zapret-strategy.why":   {"%s did not open by any method even though the engine is running.", "%s hiçbir yöntemle açılmadı, oysa motor çalışıyor."},
		"adv.zapret-strategy.s1":    {"blockcheck searches for a strategy that matches your ISP and writes it into the config", "blockcheck ISS'ne uyan stratejiyi arar ve config'e yazar"},
		"adv.zapret-strategy.gain":  {"With the right strategy the blocked targets open", "Doğru strateji ile engelli hedefler açılır"},

		"adv.hostlist-prune.title": {"autohostlist has grown to %d domains - prune it", "autohostlist %d alan adına şişmiş - temizle"},
		"adv.hostlist-prune.why":   {"As the automatic list collects false positives, traffic that is not blocked at all goes through desync. That costs CPU and latency and can break sites.", "Otomatik liste yanlış pozitif topladıkça engelli olmayan trafik de desync'ten geçer. Bu hem CPU hem gecikme maliyeti üretir ve bazı siteleri bozabilir."},
		"adv.hostlist-prune.s1":    {"The list refills; genuinely blocked names come back quickly", "Liste yeniden dolacak; gerçekten engelli olanlar kısa sürede geri gelir"},
		"adv.hostlist-prune.gain":  {"Fewer packets processed, lower latency, fewer side effects", "Daha az işlenen paket, daha düşük gecikme, daha az yan etki"},

		"adv.hostlist-manual.title": {"Switch the hostlist to manual mode", "Hostlist'i manuel moda al"},
		"adv.hostlist-manual.why":   {"The automatic list has reached %d entries and keeps growing. In manual mode only the names you choose go through desync, which is both faster and predictable.", "Otomatik liste %d girdiye ulaştı ve büyümeye devam ediyor. Manuel modda yalnızca senin seçtiğin adlar desync'ten geçer; bu hem daha hızlı hem öngörülebilir."},
		"adv.hostlist-manual.s1":    {"Copy the names you actually need into hostlist.txt", "Gerçekten gereken adları hostlist.txt içine kopyala"},
		"adv.hostlist-manual.gain":  {"Predictable behaviour and the least possible traffic through the engine", "Öngörülebilir davranış ve motordan geçen en az trafik"},

		"adv.gateway-off.title": {"Turn gateway mode off if you are not using it", "Ağ geçidi modunu kullanmıyorsan kapat"},
		"adv.gateway-off.why":   {"This machine is NATing and desyncing the traffic of the other devices on the LAN. Conntrack and CPU load rise, and NFQUEUE starts risking drops.", "Bu makine LAN'daki diğer cihazların trafiğini de NAT'layıp desync'ten geçiriyor. conntrack ve CPU yükü artar, NFQUEUE'da düşme riski doğar."},
		"adv.gateway-off.gain":  {"Less load, fewer dropped packets", "Daha az yük, daha az paket düşmesi"},

		"adv.nfqueue-drops.title": {"NFQUEUE is dropping packets - this is the local source of the instability", "NFQUEUE paket düşürüyor - kararsızlığın yerel kaynağı bu"},
		"adv.nfqueue-drops.why":   {"%d packets were dropped in the queue. When the desync engine cannot keep up, packets disappear silently and produce outages that look like an ISP fault.", "%d paket kuyrukta düştü. Desync motoru trafiğe yetişemediğinde paketler sessizce kaybolur ve bu, ISS kaynaklı gibi görünen kopmalar üretir."},
		"adv.nfqueue-drops.s1":    {"Narrow the hostlist (the advice above)", "Hostlist'i daralt (yukarıdaki öneri)"},
		"adv.nfqueue-drops.s2":    {"Turn gateway mode off", "Gateway modunu kapat"},
		"adv.nfqueue-drops.s3":    {"Take 443 out of PORTS_UDP so QUIC does not go through the engine", "PORTS_UDP'den 443'ü çıkarıp QUIC'i motordan geçirme"},
		"adv.nfqueue-drops.gain":  {"The random outages stop", "Rastgele kopmalar biter"},

		"adv.quic.title": {"QUIC (UDP/443) does not get through anywhere", "QUIC (UDP/443) hiçbir hedefte geçmiyor"},
		"adv.quic.why":   {"The browser tries QUIC, fails, and falls back to TCP, which adds a visible delay to every new connection.", "Tarayıcı QUIC deneyip başarısız olunca TCP'ye düşer; bu, her yeni bağlantıda gözle görülür bir gecikme ekler."},
		"adv.quic.s1":    {"If zapret is processing UDP/443, take it out of the list", "zapret UDP/443'ü işliyorsa listeden çıkar"},
		"adv.quic.s2":    {"If it still does not pass, the block is on the ISP side; disabling QUIC in the browser reduces the delay", "Hâlâ geçmiyorsa engel ISS tarafındadır; tarayıcıda QUIC'i kapatmak gecikmeyi azaltır"},
		"adv.quic.gain":  {"Lower first-connection latency on page loads", "Sayfa açılışlarında ilk bağlantı gecikmesi düşer"},

		// --- isp -------------------------------------------------------------------
		"adv.isp-evidence.title": {"The loss starts in the ISP backbone - report it with evidence", "Kayıp ISS omurgasında başlıyor - kanıtla birlikte bildir"},
		"adv.isp-evidence.why":   {"Loss is continuous from hop %d (%s) onwards. Everything past that point is out of your control.", "%d. atlamadan (%s) itibaren kayıp sürekli. Bu noktadan sonrası senin kontrolünde değil."},
		"adv.isp-evidence.s1":    {"Send the report to your ISP together with the traceroute table and the hourly outage distribution", "Raporu ISS'ye ilet; traceroute tablosu ve saatlik kesinti dağılımı ile birlikte"},
		"adv.isp-evidence.s2":    {"Run a long monitor to prove when the outages happen", "Kesintilerin saatini kanıtlamak için uzun süreli izleme çalıştır"},
		"adv.isp-evidence.gain":  {"Tickets opened with concrete measurements are resolved much faster", "Somut ölçümle açılan kayıtlar çok daha hızlı sonuçlanır"},

		"adv.high-rtt.title": {"Baseline latency is high (%.0f ms)", "Temel gecikme yüksek (%.0f ms)"},
		"adv.high-rtt.why":   {"Unless this is a mobile or satellite link, the traffic may be taking a longer route than it needs to.", "Mobil/uydu bağlantı değilse trafik gereğinden uzun bir yol izliyor olabilir."},
		"adv.high-rtt.s1":    {"Look at which hop the latency jumps", "Hangi atlamada sıçradığına bak"},
		"adv.high-rtt.s2":    {"If a VPN or proxy is on, turn it off and measure again", "VPN/proxy açıksa kapatıp tekrar ölç"},

		// --- security / firewall ------------------------------------------------------
		"adv.ufw-icmp.title": {"The firewall is dropping ICMP errors", "Güvenlik duvarı ICMP hatalarını düşürüyor"},
		"adv.ufw-icmp.why":   {"%d blocked ICMP messages were seen in the kernel log. Blocking 'fragmentation needed' breaks path MTU discovery, which shows up as pages that start loading and then hang.", "Çekirdek günlüğünde %d engellenmiş ICMP mesajı görüldü. 'fragmentation needed' mesajını engellemek yol MTU keşfini bozar; bu da açılmaya başlayıp donan sayfalar olarak görünür."},
		"adv.ufw-icmp.s1":    {"Allow ICMP type 3 (destination unreachable) inbound at minimum", "En azından gelen ICMP tip 3 (destination unreachable) trafiğine izin ver"},
		"adv.ufw-icmp.s2":    {"Type 11 (time exceeded) only affects traceroute, so it is safe to leave blocked", "Tip 11 (time exceeded) yalnızca traceroute'u etkiler, engelli kalabilir"},
		"adv.ufw-icmp.gain":  {"PMTU discovery works and the hanging pages stop", "PMTU keşfi çalışır, takılan sayfalar düzelir"},

		// --- ipv6 ---------------------------------------------------------------------
		"adv.ipv6-broken.title":  {"IPv6 is configured but not working", "IPv6 yapılandırılmış ama çalışmıyor"},
		"adv.ipv6-broken.why":    {"The machine has an IPv6 address but IPv6 destinations do not answer. Applications try IPv6 first, wait for the timeout, then fall back to IPv4 - that wait is added to every connection.", "Makinede IPv6 adresi var ama IPv6 hedefleri yanıt vermiyor. Uygulamalar önce IPv6 deneyip zaman aşımını bekliyor, sonra IPv4'e düşüyor - bu bekleme her bağlantıya ekleniyor."},
		"adv.ipv6-broken.s1":     {"Either fix IPv6 on the router, or disable it on this machine", "Ya router'da IPv6'yı düzelt ya da bu makinede kapat"},
		"adv.ipv6-broken.s2":     {"To disable: sysctl -w net.ipv6.conf.all.disable_ipv6=1", "Kapatmak için: sysctl -w net.ipv6.conf.all.disable_ipv6=1"},
		"adv.ipv6-broken.gain":   {"The dead-IPv6 wait disappears from every new connection", "Her yeni bağlantıdaki ölü IPv6 beklemesi kalkar"},
		"adv.ipv6-missing.title": {"IPv6 is not available", "IPv6 kullanılamıyor"},
		"adv.ipv6-missing.why":   {"No IPv6 connectivity was found. This is not a fault by itself, but some services are IPv6-only and CGNAT problems are usually solved by IPv6.", "IPv6 bağlantısı bulunamadı. Tek başına arıza değil, ama bazı servisler yalnızca IPv6 üzerinden çalışır ve CGNAT sorunları genelde IPv6 ile çözülür."},
		"adv.ipv6-missing.s1":    {"Check whether your ISP offers IPv6 and enable it on the modem", "ISS'nin IPv6 verip vermediğine bak ve modemde etkinleştir"},

		// --- method -----------------------------------------------------------------------
		"adv.measure-first.title": {"Measure before and after every change", "Her ayar değişikliğini önce/sonra ölç"},
		"adv.measure-first.why":   {"Most internet-improvement advice becomes folklore because nobody measures it. That is the reason this tool exists.", "İnternet iyileştirme önerilerinin çoğu ölçülmediği için efsane hâline gelir. Bu aracın varlık sebebi bu."},
		"adv.measure-first.s1":    {"Before the change", "Değişiklikten önce"},
		"adv.measure-first.s2":    {"Apply the setting", "Ayarı uygula"},
		"adv.measure-first.s3":    {"After: look at p95 latency and the retransmission rate", "Sonra: p95 gecikme ve yeniden gönderim oranına bak"},
		"adv.measure-first.gain":  {"You keep the settings that actually work and drop the rest", "Gerçekten işe yarayan ayarları tutar, gerisini atarsın"},

		"adv.baseline.title": {"Take a baseline while things are good", "İşler yolundayken bir referans al"},
		"adv.baseline.why":   {"The score is %.1f right now. Saving this as a baseline means the next run tells you what changed rather than just what it measured.", "Puan şu an %.1f. Bunu referans olarak kaydedersen bir sonraki çalışma sana yalnızca ne ölçtüğünü değil, neyin değiştiğini söyler."},
		"adv.baseline.s1":    {"Press b in the interface, or run: nabiz quick --baseline", "Arayüzde b tuşuna bas ya da çalıştır: nabiz quick --baseline"},
		"adv.baseline.gain":  {"Later runs are compared against this one automatically", "Sonraki çalışmalar otomatik olarak bununla karşılaştırılır"},

		"adv.monitor-long.title": {"Leave the monitor running for a few hours", "İzlemeyi birkaç saat açık bırak"},
		"adv.monitor-long.why":   {"Outages that happen every few minutes are visible in a short run, but the ones that happen twice a day are not. The event log answers 'when does it break' with timestamps.", "Birkaç dakikada bir olan kesintiler kısa testte görünür ama günde iki kez olanlar görünmez. Olay günlüğü 'ne zaman bozuluyor' sorusunu zaman damgasıyla cevaplar."},
		"adv.monitor-long.s1":    {"Then read the hourly distribution and the event log", "Sonra saatlik dağılımı ve olay günlüğünü oku"},
		"adv.monitor-long.gain":  {"Patterns like 'every evening at nine' become visible", "'Her akşam dokuzda' gibi desenler görünür hâle gelir"},
	})
}

// Advice derived from an A/B comparison rather than a single run.
func init() {
	register(map[string][2]string{
		"adv.hostlist-fp.title": {"Remove %d hostlist entries that do not need the bypass", "Bypass'a ihtiyaç duymayan %d hostlist girdisini kaldır"},
		"adv.hostlist-fp.why": {
			"With the engine stopped, %s and %d others opened cleanly on their own. They are in the hostlist anyway, so every connection to them is processed by the desync engine for nothing — CPU, latency and a chance of breaking a site that was never blocked.",
			"Motor durdurulmuşken %s ve %d tanesi kendi başına sorunsuz açıldı. Yine de hostlist'te oldukları için onlara giden her bağlantı boşuna desync'ten geçiyor — CPU, gecikme ve hiç engelli olmayan bir siteyi bozma riski."},
		"adv.hostlist-fp.s1":   {"Removes exactly those names from hostlist.txt and autohostlist.txt, nothing else", "Yalnızca o adları hostlist.txt ve autohostlist.txt içinden siler, başka bir şeye dokunmaz"},
		"adv.hostlist-fp.s2":   {"If one turns out to be blocked later, the auto list learns it again", "Sonradan biri engellenirse otomatik liste onu tekrar öğrenir"},
		"adv.hostlist-fp.gain": {"Less traffic through the engine, lower latency, fewer side effects", "Motordan geçen trafik azalır, gecikme düşer, yan etki azalır"},
	})
}

// Advice added in 0.3.2.
func init() {
	register(map[string][2]string{
		"adv.hostlist-dead.title": {"Drop %d hostlist entries that no longer resolve", "Artık çözümlenmeyen %d hostlist girdisini at"},
		"adv.hostlist-dead.why": {
			"%s and the rest did not resolve at all. A name that no longer exists cannot be blocked, so its only remaining effect is a lookup on every match and a longer list to walk.",
			"%s ve diğerleri hiç çözümlenemedi. Var olmayan bir ad engellenemez; geriye kalan tek etkisi her eşleşmede bir sorgu ve gezilecek daha uzun bir listedir."},
		"adv.hostlist-dead.s1":   {"Removes exactly those names, nothing else", "Yalnızca o adları siler, başka bir şeye dokunmaz"},
		"adv.hostlist-dead.gain": {"A shorter list and one less lookup per match", "Daha kısa liste, eşleşme başına bir sorgu daha az"},

		"adv.unwall-verify.title": {"Check whether the bypass is still doing anything", "Bypass'ın hâlâ bir işe yarayıp yaramadığını ölç"},
		"adv.unwall-verify.why": {
			"All %d probed domains opened cleanly with the engine running, which proves it is not breaking anything — but not that it is needed. Only a run with the engine stopped can tell those apart.",
			"Motor çalışırken denenen %d alan adının hepsi sorunsuz açıldı; bu, motorun bir şeyi bozmadığını kanıtlar ama gerekli olduğunu kanıtlamaz. Bunları ancak motor durdurulmuş bir koşu ayırt edebilir."},
		"adv.unwall-verify.s1":   {"Anything still clean without it does not need to be in the hostlist", "Onsuz da temiz kalan her şeyin hostlist'te işi yoktur"},
		"adv.unwall-verify.gain": {"Either a shorter hostlist, or evidence that the engine is earning its place", "Ya daha kısa bir hostlist ya da motorun yerini hak ettiğine dair kanıt"},

		"adv.nfqueue-qlen.title": {"Give the desync queue more room", "Desync kuyruğuna daha fazla alan ver"},
		"adv.nfqueue-qlen.why": {
			"%d packets were dropped in the NFQUEUE. The engine is not keeping up with the traffic being handed to it, and a dropped packet there looks exactly like packet loss from the ISP.",
			"NFQUEUE'da %d paket düştü. Motor kendisine verilen trafiğe yetişemiyor ve orada düşen bir paket, ISS kaynaklı kayıptan ayırt edilemez."},
		"adv.nfqueue-qlen.s1":   {"Then narrow what reaches the queue at all: hostlist and ports", "Sonra kuyruğa ulaşanı daralt: hostlist ve portlar"},
		"adv.nfqueue-qlen.s2":   {"Turning gateway mode off removes every other device's traffic from it", "Ağ geçidi modunu kapatmak diğer cihazların trafiğini kuyruktan tamamen çıkarır"},
		"adv.nfqueue-qlen.gain": {"The random outages that look like an ISP fault stop", "ISS arızası gibi görünen rastgele kopmalar biter"},

		"adv.bpftune-tuner-off.title": {"Stop the buffer tuner rather than fighting it", "Tampon ayarlayıcısıyla çekişmek yerine onu durdur"},
		"adv.bpftune-tuner-off.why": {
			"bpftune has raised tcp_rmem %d times, currently to %s. Pinning the value works until its next restart, at which point it starts climbing again. Running only the tuners you want settles it.",
			"bpftune tcp_rmem'i %d kez yükseltti, şu an %s. Değeri sabitlemek bir sonraki yeniden başlatmaya kadar işe yarar, sonra tekrar tırmanmaya başlar. Yalnızca istediğin ayarlayıcıları çalıştırmak bunu bitirir."},
		"adv.bpftune-tuner-off.s1":   {"Keep the congestion-control tuner, drop the buffer one:", "Tıkanıklık kontrolü ayarlayıcısını tut, tampon olanı bırak:"},
		"adv.bpftune-tuner-off.s2":   {"Set ExecStart to: /usr/sbin/bpftune -a tcp_conn_tuner", "ExecStart satırını şu yap: /usr/sbin/bpftune -a tcp_conn_tuner"},
		"adv.bpftune-tuner-off.gain": {"The buffer stays where you put it", "Tampon koyduğun yerde kalır"},
	})
}

// Wording adjusted now that the tuner change is applied through a drop-in.
func init() {
	register(map[string][2]string{
		"adv.bpftune-tuner-off.s2": {"A drop-in overrides ExecStart without touching the packaged unit", "Drop-in dosyası paket birimini değiştirmeden ExecStart'ı geçersiz kılar"},
	})
}

// The leak fix names the scope it applies to.
func init() {
	register(map[string][2]string{
		"adv.dns-leak.why": {
			"systemd-resolved keeps a resolver list per link as well as globally, and these scopes still hold a plaintext one: %s. A single timeout on the encrypted resolver is enough to fall back to them.",
			"systemd-resolved genel listenin yanında link başına da çözücü tutar ve şu kapsamlarda hâlâ düz metin bir tane var: %s. Şifreli çözücüde tek bir zaman aşımı, onlara düşmeye yeter."},
		"adv.dns-leak.s3": {"%s got these from its DHCP lease, so the fix is to stop accepting them there", "%s bunları DHCP kirasından aldı; çözüm onları orada kabul etmeyi bırakmak"},
	})
}

// Repairing a unit this tool broke.
func init() {
	register(map[string][2]string{
		"adv.bpftune-repair.title": {"bpftune is not running — it failed to start", "bpftune çalışmıyor — başlatılamadı"},
		"adv.bpftune-repair.why": {
			"The unit tried to start and exited (%s). Until it starts, nothing it was tuning is being tuned, and the recommendations about its behaviour are about a daemon that is not there.",
			"Birim başlamayı deneyip çıktı (%s). Başlayana kadar ayarladığı hiçbir şey ayarlanmıyor ve davranışıyla ilgili öneriler var olmayan bir servisi anlatıyor."},
		"adv.bpftune-repair.s1":   {"An override is replacing the packaged ExecStart; remove it first", "Bir geçersiz kılma paketin ExecStart satırını değiştiriyor; önce onu kaldır"},
		"adv.bpftune-repair.gain": {"The daemon starts again and its state can be judged at all", "Servis tekrar başlar ve durumu hakkında bir şey söylenebilir hâle gelir"},
	})
}

// The before/after workflow is the baseline feature.
func init() {
	register(map[string][2]string{
		"adv.measure-first.s0": {"Pin the current state as the reference (the baseline button, or:)", "Mevcut durumu referans olarak sabitle (referans al düğmesi ya da:)"},
		"adv.measure-first.why": {
			"Most internet-improvement advice becomes folklore because nobody measures it. Pin a baseline and every later result carries a delta against it, so a change either shows up or it did not do anything.",
			"İnternet iyileştirme önerilerinin çoğu ölçülmediği için efsane hâline gelir. Bir referans sabitlersen sonraki her sonuç ona göre farkı taşır; böylece bir değişiklik ya görünür ya da hiçbir şey yapmamıştır."},
	})
}
