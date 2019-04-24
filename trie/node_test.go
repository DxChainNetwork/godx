package trie

import "testing"

//_______ _________          _______  _______   _________ _______  _______ _________ _______
//(  ____ \\__   __/|\     /|(  ____ \(  ____ )  \__   __/(  ____ \(  ____ \\__   __/(  ____ \
//| (    \/   ) (   | )   ( || (    \/| (    )|     ) (   | (    \/| (    \/   ) (   | (    \/
//| (__       | |   | (___) || (__    | (____)|     | |   | (__    | (_____    | |   | (_____
//|  __)      | |   |  ___  ||  __)   |     __)     | |   |  __)   (_____  )   | |   (_____  )
//| (         | |   | (   ) || (      | (\ (        | |   | (            ) |   | |         ) |
//| (____/\   | |   | )   ( || (____/\| ) \ \__     | |   | (____/\/\____) |   | |   /\____) |
//(_______/   )_(   |/     \|(_______/|/   \__/     )_(   (_______/\_______)   )_(   \_______)

func TestCanUnload(t *testing.T) {
	tests := []struct {
		flag                 nodeFlag
		cachegen, cachelimit uint16
		want                 bool
	}{
		{
			flag: nodeFlag{dirty: true, gen: 0},
			want: false,
		},
		{
			flag:     nodeFlag{dirty: false, gen: 0},
			cachegen: 0, cachelimit: 0,
			want: true,
		},
		{
			flag:     nodeFlag{dirty: false, gen: 65534},
			cachegen: 65535, cachelimit: 1,
			want: true,
		},
		{
			flag:     nodeFlag{dirty: false, gen: 65534},
			cachegen: 0, cachelimit: 1,
			want: true,
		},
		{
			flag:     nodeFlag{dirty: false, gen: 1},
			cachegen: 65535, cachelimit: 1,
			want: true,
		},
	}

	for _, test := range tests {
		if got := test.flag.canUnload(test.cachegen, test.cachelimit); got != test.want {
			t.Errorf("%+v\n   got %t, want %t", test, got, test.want)
		}
	}
}
