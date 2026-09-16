package ipgeo

import "testing"

func TestClassifyRoute(t *testing.T) {
	cases := []struct {
		name     string
		hops     []HopMeta
		expected string
	}{
		{
			name: "CN2 GIA",
			hops: []HopMeta{
				{IP: "192.168.1.1"},
				{IP: "59.43.18.5", ASNumber: "AS4809"},
				{IP: "59.43.245.1", ASNumber: "AS4809"},
			},
			expected: "电信 CN2 GIA",
		},
		{
			name: "CN2 GT",
			hops: []HopMeta{
				{IP: "59.43.18.5", ASNumber: "AS4809"},
				{IP: "202.97.12.1", ASNumber: "AS4134"},
			},
			expected: "电信 CN2 GT",
		},
		{
			name: "163",
			hops: []HopMeta{
				{IP: "202.97.12.1", ASNumber: "AS4134"},
			},
			expected: "电信 163",
		},
		{
			name: "Unicom 9929",
			hops: []HopMeta{
				{IP: "210.14.3.1", ASNumber: "AS9929"},
			},
			expected: "联通 9929 (A网)",
		},
		{
			name: "Unicom 10099 to 4837",
			hops: []HopMeta{
				{IP: "103.214.12.1", ASNumber: "AS10099"},
				{IP: "219.158.3.1", ASNumber: "AS4837"},
			},
			expected: "联通 10099->4837",
		},
		{
			name: "Mobile CMIN2",
			hops: []HopMeta{
				{IP: "2402:4f00:f000::1", ASNumber: "AS58807"},
			},
			expected: "移动 CMIN2",
		},
		{
			name: "Mobile CMI",
			hops: []HopMeta{
				{IP: "223.120.1.1", ASNumber: "AS58453"},
			},
			expected: "移动 CMI",
		},
	}

	for _, c := range cases {
		got := ClassifyRoute(c.hops, "", "")
		if got != c.expected {
			t.Errorf("case %s: expected %s, got %s", c.name, c.expected, got)
		}
	}
}
