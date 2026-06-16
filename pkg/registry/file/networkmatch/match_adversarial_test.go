package networkmatch

import "testing"

// Black-box adversarial regression guards for the networkmatch public API.
//
// These tests are SECURITY guards, not contract pins. Each case is crafted in
// the FALSE-NEGATIVE direction: an observed value that an attacker would want
// silently admitted by a profile entry it should NOT match (an over-broadening
// that would let unauthorized egress/ingress slip past R0005/R0011/R1003/R1009).
// Every adversarial case is paired with a positive control proving the matcher
// is not simply rejecting everything.
//
// All probing is through the real public surface: MatchIP / CompileIP+Match,
// MatchDNS / CompileDNS+Match, ValidateIPEntry / ValidateDNSEntry. No internals.
//
// Where an adversarial probe revealed real CURRENT behavior worth pinning, the
// case is marked with a "KNOWN GAP" comment and asserts the current output so
// the suite stays green while the behavior is documented (see the final report).

// --- IP: CIDR boundary, no silent over-broadening ---------------------------

func TestAdversarial_IP_CIDRBoundaryNoOverBroaden(t *testing.T) {
	cases := []struct {
		name     string
		profile  []string
		observed string
		want     bool
	}{
		// /24: 10.0.0.0 .. 10.0.0.255 inclusive. The attacker probes the first
		// address OUTSIDE the mask. It MUST be rejected.
		{"slash24-just-inside-broadcast", []string{"10.0.0.0/24"}, "10.0.0.255", true}, // control
		{"slash24-just-outside-next-net", []string{"10.0.0.0/24"}, "10.0.1.0", false},  // adversarial
		// /25 splits the third octet's low half. .127 in, .128 out.
		{"slash25-just-inside", []string{"10.0.0.0/25"}, "10.0.0.127", true},   // control
		{"slash25-just-outside", []string{"10.0.0.0/25"}, "10.0.0.128", false}, // adversarial
		// /31 point-to-point: exactly two addresses.
		{"slash31-low-in", []string{"10.0.0.0/31"}, "10.0.0.0", true},   // control
		{"slash31-high-in", []string{"10.0.0.0/31"}, "10.0.0.1", true},  // control
		{"slash31-just-out", []string{"10.0.0.0/31"}, "10.0.0.2", false}, // adversarial
		// /8 upper boundary: 10.255.255.255 in, 11.0.0.0 out.
		{"slash8-top-in", []string{"10.0.0.0/8"}, "10.255.255.255", true}, // control
		{"slash8-just-out", []string{"10.0.0.0/8"}, "11.0.0.0", false},    // adversarial
		// IPv6 /64 boundary.
		{"v6-slash64-in", []string{"2001:db8:0:1::/64"}, "2001:db8:0:1:ffff::1", true},  // control
		{"v6-slash64-out", []string{"2001:db8:0:1::/64"}, "2001:db8:0:2::1", false},     // adversarial
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchIP(tc.profile, tc.observed); got != tc.want {
				t.Errorf("MatchIP(%v, %q) = %v, want %v", tc.profile, tc.observed, got, tc.want)
			}
		})
	}
}

// TestAdversarial_IP_LeadingZerosNotOctal guards against the classic SSRF/ACL
// bypass where "010.0.0.1" is parsed as octal (= 8.0.0.1) by some libraries.
// Go's net.ParseIP rejects leading-zero octets outright, so the observation is
// garbage and matches nothing — including the "*" sentinel.
func TestAdversarial_IP_LeadingZerosNotOctal(t *testing.T) {
	if MatchIP([]string{"10.0.0.1"}, "010.000.000.001") {
		t.Error("leading-zero octets must NOT be reinterpreted as a literal match")
	}
	// Sentinel must also refuse it — a garbage observation is not a valid IP.
	if MatchIP([]string{"*"}, "010.000.000.001") {
		t.Error("'*' must NOT admit a leading-zero (octal-ambiguous) observation")
	}
	// Control: the canonical form does match.
	if !MatchIP([]string{"10.0.0.1"}, "10.0.0.1") {
		t.Error("canonical literal must match (positive control)")
	}
}

// TestAdversarial_IP_MappedV6DoesNotEscapeMask checks that the IPv4-mapped IPv6
// representation of an address OUTSIDE a CIDR is still rejected. An attacker
// must not be able to dodge "10.0.0.0/8" by sending "::ffff:11.0.0.1".
func TestAdversarial_IP_MappedV6DoesNotEscapeMask(t *testing.T) {
	// Control: in-range address, expressed in mapped-v6 form, matches.
	if !MatchIP([]string{"10.0.0.0/8"}, "::ffff:10.1.2.3") {
		t.Error("in-range mapped-v6 address must match the v4 CIDR (positive control)")
	}
	// Adversarial: out-of-range address in mapped-v6 form must NOT match.
	if MatchIP([]string{"10.0.0.0/8"}, "::ffff:11.0.0.1") {
		t.Error("out-of-range mapped-v6 address must NOT slip past the v4 CIDR")
	}
}

// TestAdversarial_IP_FamilyConfusion guards the address-family separation:
// a v4-only any-CIDR must not admit any v6 traffic and vice-versa, so an
// attacker can't use a v6 source to bypass a v4-scoped allow rule.
func TestAdversarial_IP_FamilyConfusion(t *testing.T) {
	// 0.0.0.0/0 is "any v4". A v6 observation must NOT be admitted.
	if MatchIP([]string{"0.0.0.0/0"}, "2001:db8::1") {
		t.Error("0.0.0.0/0 must NOT admit IPv6 traffic")
	}
	// ::/0 is "any v6". A pure v4 observation must NOT be admitted.
	if MatchIP([]string{"::/0"}, "203.0.113.7") {
		t.Error("::/0 must NOT admit IPv4 traffic")
	}
	// Controls.
	if !MatchIP([]string{"0.0.0.0/0"}, "203.0.113.7") {
		t.Error("0.0.0.0/0 must admit IPv4 (positive control)")
	}
	if !MatchIP([]string{"::/0"}, "2001:db8::1") {
		t.Error("::/0 must admit IPv6 (positive control)")
	}
}

// TestAdversarial_IP_SentinelStillNeedsValidObservation ensures the "*" any
// sentinel does not degrade into "match literally anything (including non-IP
// junk)". A profile that allows any IP must still reject garbage observations,
// so a corrupted/forged non-IP event cannot be laundered into an allow.
func TestAdversarial_IP_SentinelStillNeedsValidObservation(t *testing.T) {
	bad := []string{"", "not-an-ip", "10.0.0.0/8", "10.0.0.256", "1.2.3.4.5", "::ffff:zzzz"}
	for _, obs := range bad {
		if MatchIP([]string{"*"}, obs) {
			t.Errorf("'*' must NOT admit non-IP observation %q", obs)
		}
	}
	// Control: a real IP is admitted by the sentinel.
	if !MatchIP([]string{"*"}, "8.8.8.8") {
		t.Error("'*' must admit a valid IP (positive control)")
	}
}

// TestAdversarial_IP_MalformedEntryNeverWidens ensures a malformed profile
// entry is dropped, never reinterpreted into a broad match. An attacker who can
// influence a profile entry must not be able to turn "10.0.0.0/40" (invalid
// mask) into an accidental any-match.
func TestAdversarial_IP_MalformedEntryNeverWidens(t *testing.T) {
	cases := []struct {
		name    string
		profile []string
	}{
		{"oversize-mask", []string{"10.0.0.0/40"}},
		{"negative-mask", []string{"10.0.0.0/-1"}},
		{"garbage-cidr-host", []string{"not-an-ip/8"}},
		{"trailing-junk", []string{"10.0.0.0/8 evil"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// An out-of-any-plausible-range observation must stay rejected.
			if MatchIP(tc.profile, "203.0.113.7") {
				t.Errorf("malformed entry %v must not admit 203.0.113.7", tc.profile)
			}
			// Even an in-the-prefix observation must stay rejected — the entry
			// is invalid, so it contributes nothing.
			if MatchIP(tc.profile, "10.1.2.3") {
				t.Errorf("malformed entry %v must contribute no coverage", tc.profile)
			}
		})
	}
}

// --- IP via the compiled path -----------------------------------------------

// TestAdversarial_IP_CompiledPathAgreesWithWrapper guards that CompileIP+Match
// (the hot path actually used by the CEL functions) does not over-broaden where
// the MatchIP wrapper holds firm. A divergence here is a real exploit surface.
func TestAdversarial_IP_CompiledPathAgreesWithWrapper(t *testing.T) {
	m := CompileIP([]string{"10.0.0.0/24", "192.168.1.5"})
	if m.Match("10.0.1.0") {
		t.Error("compiled matcher must NOT admit just-outside-/24 address")
	}
	if m.Match("192.168.1.6") {
		t.Error("compiled matcher must NOT admit a sibling of the literal")
	}
	if !m.Match("10.0.0.42") { // control
		t.Error("compiled matcher must admit in-range /24 address")
	}
	if !m.Match("192.168.1.5") { // control
		t.Error("compiled matcher must admit the exact literal")
	}
}

// --- DNS: wildcard label-count guards ---------------------------------------

// TestAdversarial_DNS_LeadingStarOneLabelOnly is the headline DNS over-broadening
// guard. "*.example.com." authorizes ONE subdomain label. The attacker tries:
//   - the apex (zero labels) — a different security zone,
//   - a two-label-deep name (api.evil.example.com.) — could be an attacker-
//     controlled sub-sub-domain,
//   - a sibling registrable domain (example.com.attacker.com.).
// All must be rejected.
func TestAdversarial_DNS_LeadingStarOneLabelOnly(t *testing.T) {
	cases := []struct {
		name     string
		observed string
		want     bool
	}{
		{"one-label-control", "api.example.com.", true},          // control
		{"apex-zero-labels", "example.com.", false},              // adversarial
		{"two-labels-deep", "api.evil.example.com.", false},      // adversarial
		{"suffix-as-prefix-of-attacker", "x.example.com.attacker.com.", false}, // adversarial
		{"empty-label-injection", "a..example.com.", false},      // adversarial (malformed obs)
		{"leading-dot-observation", ".api.example.com.", false},  // adversarial (empty first label)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchDNS([]string{"*.example.com."}, tc.observed); got != tc.want {
				t.Errorf("MatchDNS(['*.example.com.'], %q) = %v, want %v", tc.observed, got, tc.want)
			}
		})
	}
}

// TestAdversarial_DNS_SuffixSubstringNotMatched guards against a substring/suffix
// confusion: "stripe.com." must not be admitted by a profile for "notstripe.com."
// nor by the reverse, and "evilstripe.com." must not satisfy "stripe.com.".
// Label-anchored matching, not raw string suffix.
func TestAdversarial_DNS_SuffixSubstringNotMatched(t *testing.T) {
	cases := []struct {
		name     string
		profile  []string
		observed string
		want     bool
	}{
		{"control-exact", []string{"stripe.com."}, "stripe.com.", true},
		{"prefix-glued-label", []string{"stripe.com."}, "evilstripe.com.", false},
		{"suffix-glued-label", []string{"stripe.com."}, "stripe.community.", false},
		{"wildcard-prefix-glued", []string{"*.stripe.com."}, "api.evilstripe.com.", false},
		{"wildcard-suffix-confusion", []string{"*.stripe.com."}, "api.stripe.com.attacker.net.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchDNS(tc.profile, tc.observed); got != tc.want {
				t.Errorf("MatchDNS(%v, %q) = %v, want %v", tc.profile, tc.observed, got, tc.want)
			}
		})
	}
}

// TestAdversarial_DNS_TrailingStarNeverZero guards the project extension:
// "internal.*" allows ONE OR MORE labels after "internal", never zero. The bare
// "internal." apex must be rejected, and a different prefix must be rejected.
func TestAdversarial_DNS_TrailingStarNeverZero(t *testing.T) {
	cases := []struct {
		name     string
		observed string
		want     bool
	}{
		{"one-label-control", "internal.svc.", true},          // control
		{"many-labels-control", "internal.a.b.c.", true},      // control
		{"zero-labels-apex", "internal.", false},              // adversarial
		{"different-prefix", "external.svc.", false},          // adversarial
		{"prefix-as-substring", "internalx.svc.", false},      // adversarial
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchDNS([]string{"internal.*"}, tc.observed); got != tc.want {
				t.Errorf("MatchDNS(['internal.*'], %q) = %v, want %v", tc.observed, got, tc.want)
			}
		})
	}
}

// TestAdversarial_DNS_MidEllipsisExactlyOneLabel guards the "⋯" mid token:
// exactly one label between the anchors. Zero labels and two labels must both
// be rejected so an attacker cannot pivot through an extra subdomain.
func TestAdversarial_DNS_MidEllipsisExactlyOneLabel(t *testing.T) {
	prof := []string{"kubernetes.⋯.svc.cluster.local."}
	cases := []struct {
		name     string
		observed string
		want     bool
	}{
		{"one-label-control", "kubernetes.default.svc.cluster.local.", true}, // control
		{"zero-labels", "kubernetes.svc.cluster.local.", false},              // adversarial
		{"two-labels", "kubernetes.a.b.svc.cluster.local.", false},           // adversarial
		{"wrong-anchor-suffix", "kubernetes.default.svc.cluster.evil.", false}, // adversarial
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchDNS(prof, tc.observed); got != tc.want {
				t.Errorf("MatchDNS(%v, %q) = %v, want %v", prof, tc.observed, got, tc.want)
			}
		})
	}
}

// TestAdversarial_DNS_CaseAndTrailingDotCannotEvade guards canonicalization:
// mixing case and trailing dots must not let an attacker dodge a deny-by-default
// posture by neither matching (false negative for the defender) NOR widening.
// Here the security property is "the obvious hit still hits regardless of
// cosmetic form" — an attacker cannot evade detection by changing case/dot.
func TestAdversarial_DNS_CaseAndTrailingDotCannotEvade(t *testing.T) {
	prof := []string{"API.Stripe.COM."}
	for _, obs := range []string{
		"api.stripe.com.",
		"API.STRIPE.COM.",
		"api.stripe.com",   // no trailing dot
		"Api.Stripe.Com",   // mixed, no dot
	} {
		if !MatchDNS(prof, obs) {
			t.Errorf("case/dot variant %q must still match (no evasion via canonicalization)", obs)
		}
	}
	// Double trailing dot is a malformed name (empty terminal label) and must
	// NOT match — it is not a valid observation.
	if MatchDNS(prof, "api.stripe.com..") {
		t.Error("double-trailing-dot observation must NOT match (malformed)")
	}
}

// TestAdversarial_DNS_MalformedNeverMatches guards that malformed profile
// entries and observations contribute no coverage and never widen the set.
func TestAdversarial_DNS_MalformedNeverMatches(t *testing.T) {
	cases := []struct {
		name     string
		profile  []string
		observed string
		want     bool
	}{
		{"recursive-double-star-entry", []string{"**.example.com."}, "evil.example.com.", false},
		{"double-star-mid", []string{"a.**.b."}, "a.x.b.", false},
		{"empty-inner-label-entry", []string{"foo..bar."}, "foo.bar.", false},
		{"lone-star-entry", []string{"*"}, "anything.com.", false},
		{"empty-observation", []string{"api.stripe.com."}, "", false},
		{"whitespace-observation", []string{"api.stripe.com."}, "   ", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchDNS(tc.profile, tc.observed); got != tc.want {
				t.Errorf("MatchDNS(%v, %q) = %v, want %v", tc.profile, tc.observed, got, tc.want)
			}
		})
	}
}

// TestAdversarial_DNS_CompiledPathAgrees guards the hot CompileDNS+Match path
// against the same one-label-only over-broadening probes.
func TestAdversarial_DNS_CompiledPathAgrees(t *testing.T) {
	m := CompileDNS([]string{"*.example.com.", "internal.*"})
	if m.Match("a.b.example.com.") {
		t.Error("compiled DNS matcher must NOT admit two-label-deep under *.example.com.")
	}
	if m.Match("example.com.") {
		t.Error("compiled DNS matcher must NOT admit the apex under *.example.com.")
	}
	if m.Match("internal.") {
		t.Error("compiled DNS matcher must NOT admit zero-label internal.*")
	}
	if !m.Match("api.example.com.") { // control
		t.Error("compiled DNS matcher must admit one-label under *.example.com.")
	}
	if !m.Match("internal.svc.") { // control
		t.Error("compiled DNS matcher must admit internal.<label>")
	}
}

// --- Validation guards: admission must reject what runtime tolerates ---------

// TestAdversarial_Validate_RejectsOverBroadeningForms ensures the admission
// layer rejects the dangerous-but-runtime-tolerated forms, so the asymmetry in
// TestAdversarial_DNS_MidStarRuntimeTolerated below cannot be reached through a
// validated write path.
func TestAdversarial_Validate_RejectsOverBroadeningForms(t *testing.T) {
	mustRejectDNS := []string{
		"foo.*.bar.",       // bare "*" in a middle position
		"**",               // recursive
		"a.**.b.",          // recursive mid
		"*",                // lone star, no anchor
		"foo..bar.",        // empty inner label
		"",                 // empty
		".",                // dot only
	}
	for _, e := range mustRejectDNS {
		if err := ValidateDNSEntry(e); err == nil {
			t.Errorf("ValidateDNSEntry(%q) = nil, want rejection (over-broadening / malformed)", e)
		}
	}
	mustRejectIP := []string{
		"",                 // empty
		"not-an-ip",        // garbage
		"10.0.0.0/40",      // oversize mask
		"10.0.0.256",       // octet overflow
		"010.0.0.1",        // leading zero
		"10.0.0.0/8 evil",  // trailing junk
	}
	for _, e := range mustRejectIP {
		if err := ValidateIPEntry(e); err == nil {
			t.Errorf("ValidateIPEntry(%q) = nil, want rejection", e)
		}
	}
	// Controls: legitimate forms accepted.
	for _, e := range []string{"*", "0.0.0.0/0", "::/0", "10.0.0.0/8", "2001:db8::1"} {
		if err := ValidateIPEntry(e); err != nil {
			t.Errorf("ValidateIPEntry(%q) = %v, want nil (positive control)", e, err)
		}
	}
	for _, e := range []string{"api.stripe.com.", "*.example.com.", "internal.*", "kubernetes.⋯.svc.cluster.local."} {
		if err := ValidateDNSEntry(e); err != nil {
			t.Errorf("ValidateDNSEntry(%q) = %v, want nil (positive control)", e, err)
		}
	}
}

// TestAdversarial_DNS_MidStarRuntimeTolerated is a CHARACTERIZATION test, not a
// desired-behavior assertion.
//
// KNOWN GAP: the runtime matcher (compileDNSPattern / matchDNSPattern) treats a
// bare "*" in a MIDDLE position as a one-label wildcard, even though
// ValidateDNSEntry REJECTS that form at admission. So an entry like
// "foo.*.bar." — which can never be written through a validated path — would,
// if it ever reached the matcher (e.g. an unvalidated profile load, a storage
// bug, or a code path that skips admission), silently behave like
// "foo.⋯.bar." and admit "foo.<anything>.bar.".
//
// This is defense-in-depth asymmetry: admission is the gate, runtime is lenient.
// The CURRENT runtime behavior is pinned here so the asymmetry is visible and a
// regression (e.g. widening to multi-label) is caught. The defended invariant
// is enforced by TestAdversarial_Validate_RejectsOverBroadeningForms above.
func TestAdversarial_DNS_MidStarRuntimeTolerated(t *testing.T) {
	// Confirm the admission layer rejects it (the real protection).
	if err := ValidateDNSEntry("foo.*.bar."); err == nil {
		t.Fatal("precondition: ValidateDNSEntry must reject 'foo.*.bar.'")
	}
	// KNOWN GAP: runtime tolerates the unvalidated entry as a single-label wildcard.
	if !MatchDNS([]string{"foo.*.bar."}, "foo.x.bar.") {
		t.Error("characterization: runtime currently admits one mid label for 'foo.*.bar.'")
	}
	// The leniency is bounded to exactly one label — it does NOT recurse.
	if MatchDNS([]string{"foo.*.bar."}, "foo.x.y.bar.") {
		t.Error("characterization: mid-'*' must remain bounded to a single label, not multi")
	}
}
