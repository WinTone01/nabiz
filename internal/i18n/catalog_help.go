package i18n

// Help screen and the longer explanatory paragraphs shown inside sections.
func init() {
	register(map[string][2]string{
		"help.sec.sections": {"Sections", "Bölümler"},
		"help.sec.keys":     {"Keys", "Tuşlar"},
		"help.sec.reading":  {"Reading the results", "Sonuçlar nasıl okunur"},
		"help.sec.limits":   {"Measurement limits", "Ölçüm sınırları"},

		"help.sections.overview": {"Overview — live ping, interface counters, kernel state, tool status and the verdict from the last run.", "Genel Bakış — canlı ping, arayüz sayaçları, çekirdek durumu, araç durumu ve son çalışmanın sonucu."},
		"help.sections.test":     {"Test — pick one of the seven suites and run it; the score, findings and tables land here.", "Test — yedi ölçüm paketinden birini seç ve çalıştır; puan, bulgular ve tablolar burada."},
		"help.sections.layers":   {"Layers — physical link, TCP counters, per-socket state, netfilter and sysctl, each read straight from the kernel.", "Katmanlar — fiziksel bağlantı, TCP sayaçları, soket başına durum, netfilter ve sysctl; hepsi doğrudan çekirdekten."},
		"help.sections.kernel":   {"Kernel & Boots — link drops per kernel release, taken from wtmp so it survives journal rotation. This is what tells a driver regression apart from a broken cable.", "Çekirdek & Açılışlar — çekirdek sürümü başına link düşmesi; wtmp'den okunur, journal rotasyonundan etkilenmez. Sürücü regresyonunu bozuk kablodan ayıran şey budur."},
		"help.sections.bpftune":  {"bpftune — what the auto-tuner changed, how often, and whether the buffers it grew make sense for this line's bandwidth-delay product.", "bpftune — otomatik ayarlayıcının neyi kaç kez değiştirdiği ve büyüttüğü tamponların bu hattın bant-gecikme çarpımına göre anlamlı olup olmadığı."},
		"help.sections.unwall":   {"Unwall — zapret state, the DPI scan and the block-type classification.", "Unwall — zapret durumu, DPI taraması ve engel türü sınıflandırması."},
		"help.sections.dns":      {"DNS — plain, DoT and DoH side by side, plus the interference checks.", "DNS — düz, DoT ve DoH yan yana; ayrıca müdahale kontrolleri."},
		"help.sections.monitor":  {"Monitor — leave it running for hours; it timestamps every outage and shows the hourly distribution.", "İzleme — saatlerce açık bırak; her kesintiyi zaman damgasıyla kaydeder ve saatlik dağılımı gösterir."},
		"help.sections.advice":   {"Advice — a ranked, measurement-backed to-do list with the exact commands.", "Öneriler — ölçüme dayalı, önceliklendirilmiş, komutlu yapılacaklar listesi."},
		"help.sections.history":  {"History — the score trend across saved runs, and the baseline you pinned.", "Geçmiş — kayıtlı çalışmalar boyunca puan eğilimi ve sabitlediğin referans."},
		"help.sections.reports":  {"Reports — browse saved runs; e exports the selected one as markdown and json.", "Raporlar — kayıtlı çalışmalara göz at; e ile seçileni markdown ve json olarak dışa aktar."},

		"help.keys.nav":    {"1..0, r, ?   switch section        tab / shift+tab   cycle sections", "1..0, r, ?   bölüm değiştir        tab / shift+tab   sırayla gez"},
		"help.keys.run":    {"R or enter   run                   s                 stop", "R veya enter  çalıştır             s                 durdur"},
		"help.keys.export": {"e            export markdown+json  b                 pin this run as the baseline", "e            markdown+json dışa aktar  b            bu çalışmayı referans al"},
		"help.keys.ab":     {"a            A/B compare (on the bpftune and Unwall sections)", "a            A/B karşılaştır (bpftune ve Unwall bölümlerinde)"},
		"help.keys.lang":   {"L            switch language between English and Turkish", "L            dili İngilizce ve Türkçe arasında değiştir"},
		"help.keys.quit":   {"↑ ↓ pgup pgdn  scroll             q                 quit", "↑ ↓ pgup pgdn  kaydır              q                 çık"},

		"help.reading.1": {"A clean ping to the modem with loss beyond it means the problem is upstream of your house.", "Modeme ping temizken çıpalarda kayıp varsa sorun evin dışındadır."},
		"help.reading.2": {"Loss or jitter on the way to the modem itself means the problem is inside: cable, Wi-Fi, switch.", "Modeme giden yolda bile kayıp/jitter varsa sorun ev içindedir: kablo, Wi-Fi, switch."},
		"help.reading.3": {"A rising link-drop count means the connection is physically dropping, and no software setting fixes that. Check the Kernel & Boots section before blaming the cable: if a previous kernel was quiet for hundreds of hours, it is a regression.", "Link düşme sayısı artıyorsa bağlantı fiziksel olarak kopuyordur ve bunu hiçbir yazılım ayarı düzeltmez. Kabloyu suçlamadan önce Çekirdek & Açılışlar bölümüne bak: önceki bir çekirdek yüzlerce saat sessizse bu bir regresyondur."},
		"help.reading.4": {"Latency rising by more than 100 ms under load is bufferbloat: calls and games break even when the speed test looks fine. The fix is smarter queueing on the router, not a bigger buffer.", "Yük altında gecikme +100 ms üzerine çıkıyorsa bufferbloat vardır: hız testi iyi görünse bile oyun ve görüşme bozulur. Çözüm daha büyük tampon değil, router'da akıllı kuyruktur."},
		"help.reading.5": {"The retransmission percentage is the fastest way to separate physical loss from congestion.", "Yeniden gönderim yüzdesi, fiziksel kayıp ile tıkanıklığı ayırmanın en hızlı yoludur."},
		"help.reading.6": {"cwnd and min_rtt are read together: a small cwnd with a steady min_rtt is loss, a large cwnd with inflating rtt is queueing.", "cwnd ve min_rtt birlikte okunur: küçük cwnd + sabit min_rtt kayıp, büyük cwnd + şişen rtt kuyruk demektir."},

		"help.limits.1": {"The split-ClientHello probe only manipulates TCP segment boundaries. zapret's fake packets and TTL tricks live in the kernel and cannot be reproduced from userspace, so 'splitting helped' is a hint, not a replacement for blockcheck.", "Bölünmüş ClientHello testi yalnızca TCP segment sınırlarını oynatır. zapret'in sahte paket ve TTL numaraları çekirdek düzeyindedir ve kullanıcı alanından taklit edilemez; 'bölme işe yaradı' bir ipucudur, blockcheck'in yerine geçmez."},
		"help.limits.2": {"NFQUEUE counters and the nftables table are readable by root only.", "NFQUEUE sayaçları ve nftables tablosu yalnızca root ile okunabilir."},
		"help.limits.3": {"The throughput test uses Cloudflare endpoints: it measures real performance over that path rather than the line's peak rate.", "Hız testi Cloudflare uç noktalarını kullanır: hattın tepe hızını değil, o yoldaki gerçek performansı ölçer."},

		"help.kernel.intro":  {"Link drops counted per kernel release. Boot times and kernel versions come from wtmp, which survives journal rotation; the drop counts come from the kernel log, so older boots may show as unmeasured. A release that ran for a long time with zero drops next to one that drops every few minutes is a regression, not a cable.", "Çekirdek sürümü başına link düşmesi. Açılış zamanları ve çekirdek sürümleri wtmp'den gelir ve journal rotasyonundan etkilenmez; düşme sayıları çekirdek günlüğünden gelir, bu yüzden eski açılışlar 'kayıt yok' görünebilir. Uzun süre sıfır düşmeyle çalışmış bir sürümün yanında birkaç dakikada bir düşen bir sürüm varsa bu kablo değil, regresyondur."},
		"help.bpftune.intro": {"bpftune changes sysctl values on its own, which means /etc/sysctl.d no longer describes the running system. The table below is what its journal actually recorded, and what those values mean for this line's bandwidth-delay product.", "bpftune sysctl değerlerini kendi başına değiştirir; bu yüzden /etc/sysctl.d artık çalışan sistemi anlatmaz. Aşağıdaki tablo günlüğünde gerçekten kayıtlı olan değişiklikler ve bunların bu hattın bant-gecikme çarpımına göre anlamıdır."},
		"help.bpftune.bdp":   {"how much data can be in flight at once; the meaningful ceiling for buffers is a few times this", "aynı anda yolda olabilecek veri miktarı; tamponların anlamlı üst sınırı bunun birkaç katıdır"},
		"help.dpi.explainer": {"The TLS ClientHello is sent whole, then split at the record header and again across the SNI. If only the split version gets through, the DPI matches SNI inside a single packet and does not reassemble the TCP stream - which is exactly what zapret's multisplit and multidisorder strategies address.", "TLS ClientHello önce bütün, sonra kayıt başlığından ve SNI ortasından bölünerek gönderilir. Yalnızca bölünmüş hâli geçiyorsa DPI, SNI'yi tek pakette arıyor ve TCP akışını birleştirmiyor demektir; zapret'in multisplit ve multidisorder stratejileri tam bunun içindir."},
	})
}
