package devicetrustv1

import (
	"testing"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

func TestGetOSFromUserAgent(t *testing.T) {
	tests := []struct {
		name           string
		uas            []string
		maxTouchPoints uint32
		want           devicepb.OSType
	}{
		{
			name: "macOS desktops",
			uas: []string{
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Safari/605.1.1",               // Safari
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.1",               // Safari
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",              // Chrome/Brave
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 14.3; rv:123.0) Gecko/20100101 Firefox/123.0",                                                // Firefox
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_3_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 OPR/108.0.0.0", // Opera
			},
			want: devicepb.OSType_OS_TYPE_MACOS,
		},
		{
			name: "Windows desktops",
			uas: []string{
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.3",                                     // Chrome
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.3",                                     // Chrome
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",                                    // Chrome
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Config/91.2.2025.1",                 // Chrome
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/117.",                                                                    // Firefox
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.",                                                                    // Firefox
				"Mozilla/5.0 (Windows NT 6.1; Win64; x64; rv:109.0) Gecko/20100101 Firefox/115.",                                                                     // Firefox
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 OPR/107.0.0.0 (Edition Campaign 75", // Opera, truncated?
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 OPR/108.0.0.0",                      // Opera
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 Edg/121.0.0.",                       // Edge
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 Edg/121.0.2277.128",                 // Edge
				"Mozilla/5.0 (Windows NT 10.0; Trident/7.0; rv:11.0) like Gecko",                                                                                     // IE
				"Mozilla/5.0 (compatible; MSIE 10.0; Windows NT 6.2; Trident/6.0)",                                                                                   // IE
			},
			want: devicepb.OSType_OS_TYPE_WINDOWS,
		},
		{
			name: "Linux desktops",
			uas: []string{
				"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",               // Chrome
				"Mozilla/5.0 (X11; Linux x86_64; rv:123.0) Gecko/20100101 Firefox/123.0",                                              // Firefox
				"Mozilla/5.0 (X11; Linux i686; rv:123.0) Gecko/20100101 Firefox/123.0",                                                // Firefox
				"Mozilla/5.0 (X11; Ubuntu; Linux i686; rv:123.0) Gecko/20100101 Firefox/123.0",                                        // Firefox, Ubuntu
				"Mozilla/5.0 (X11; Fedora; Linux x86_64; rv:123.0) Gecko/20100101 Firefox/123.0",                                      // Firefox, Fedora
				"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 OPR/108.0.0.0", // Opera
			},
			want: devicepb.OSType_OS_TYPE_LINUX,
		},
		{
			name: "Mobile",
			uas: []string{
				"Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.6261.64 Mobile Safari/537.36",                              // K
				"Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36",                                         // K
				"Mozilla/5.0 (Linux; Android 10; HD1913) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.6261.64 Mobile Safari/537.36 EdgA/121.0.2277.105",     // OnePlus
				"Mozilla/5.0 (Linux; Android 10; SM-G973F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.6261.64 Mobile Safari/537.36 EdgA/121.0.2277.105",   // Samsung
				"Mozilla/5.0 (Linux; Android 10; Pixel 3 XL) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.6261.64 Mobile Safari/537.36 EdgA/121.0.2277.105", // Pixel
				"Mozilla/5.0 (Android 14; Mobile; rv:123.0) Gecko/123.0 Firefox/123.0",                                                                             // "generic" Android
			},
			want: devicepb.OSType_OS_TYPE_UNSPECIFIED,
		},
		{
			name: "Misc (UNSPECIFIED)",
			uas: []string{
				"Mozilla/5.0 (X11; CrOS x86_64 8172.45.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.64 Safari/537.36",                                  // ChromeOS
				"Mozilla/5.0 (CrKey armv7l 1.5.16041) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/31.0.1650.0 Safari/537.36",                                       // Chromecast
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64; Xbox; Xbox Series X) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/48.0.2564.82 Safari/537.36 Edge/20.02", // Xbox Series X
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64; XBOX_ONE_ED) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.79 Safari/537.36 Edge/14.14393",      // Xbox One S
				"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",                                                                           // Google bot
				"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",                                                                            // Bing bot
				"Mozilla/5.0 (compatible; Yahoo! Slurp; http://help.yahoo.com/help/us/ysearch/slurp)",                                                                // Yahoo bot
				"ceci n'est pas an user agent",
				"",
			},
			want: devicepb.OSType_OS_TYPE_UNSPECIFIED,
		},
		{
			name: "iOS",
			uas: []string{
				"Mozilla/5.0 (iPhone; CPU iPhone OS 17_3_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 EdgiOS/121.2277.133 Mobile/15E148 Safari/605.1.15", // iPhone, Safari
				"Mozilla/5.0 (iPhone; CPU iPhone OS 17_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/122.0.6261.62 Mobile/15E148 Safari/604.1",                   // iPhone, Chrome
				"Mozilla/5.0 (iPhone; CPU iPhone OS 14_3_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/123.0 Mobile/15E148 Safari/605.1.15",                      // iPhone, Firefox
			},
			want: devicepb.OSType_OS_TYPE_IOS,
		},
		{
			name:           "iPadOS",
			maxTouchPoints: 5,
			uas: []string{
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Safari/605.1.15",
				"Mozilla/5.0 (iPad; CPU OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.4 Mobile/15E148 Safari/604.1", // Request Mobile Website
			},
			want: devicepb.OSType_OS_TYPE_IPADOS,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, ua := range test.uas {
				got := getOSFromUserAgent(ua, test.maxTouchPoints)
				if got != test.want {
					t.Errorf("getOSFromUserAgent(%q) = %s, want %s", ua, got, test.want)
				}
			}
		})
	}
}
