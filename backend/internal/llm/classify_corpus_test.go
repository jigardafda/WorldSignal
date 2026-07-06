package llm

import (
	"strings"
	"testing"
)

// corpusCase is a realistic headline labeled with the taxonomy DOMAIN it should
// resolve to (the part before "."). The corpus spans every topical domain to
// guard against regressions in coverage and to enforce the categorization floor.
type corpusCase struct {
	headline string
	domain   string
}

// classificationCorpus is a broad, realistic sample of news headlines. It is the
// deterministic yardstick for the categorization goal: the heuristic classifier
// must place ≥99% of these OUTSIDE GENERAL.OTHER, and hit a strong domain-accuracy
// floor. CI cannot call the real LLM, so this hardens the deterministic floor; the
// LLM path performs at least as well in production.
var classificationCorpus = []corpusCase{
	// POLITICS
	{"President signs executive order overhauling federal agencies", "POLITICS"},
	{"Opposition party wins landslide in national election", "POLITICS"},
	{"Lawmakers debate new spending bill in parliament", "POLITICS"},
	{"Foreign minister holds summit to ease diplomatic tensions", "POLITICS"},
	{"Thousands join protest march against government", "POLITICS"},
	{"Minister resigns amid corruption scandal", "POLITICS"},
	{"Prime minister announces cabinet reshuffle", "POLITICS"},
	{"Voters head to the polls in tight presidential contest", "POLITICS"},
	{"Congress approves budget after lengthy negotiations", "POLITICS"},
	{"Referendum on constitutional change set for autumn", "POLITICS"},
	{"Senate confirms new supreme court nominee in party-line vote", "POLITICS"},

	// ECONOMY
	{"Inflation climbs to six percent as consumer prices surge", "ECONOMY"},
	{"Federal Reserve raises interest rates by fifty basis points", "ECONOMY"},
	{"Stock market tumbles as Nasdaq sheds three percent", "ECONOMY"},
	{"Unemployment falls as payrolls beat forecasts", "ECONOMY"},
	{"New tariffs on imports spark trade war fears", "ECONOMY"},
	{"GDP shrinks as the economy slips into recession", "ECONOMY"},
	{"Central bank warns of persistent inflation risk", "ECONOMY"},
	{"Bitcoin rallies past seventy thousand amid crypto surge", "ECONOMY"},
	{"Wage growth slows as the labor market cools", "ECONOMY"},
	{"Cost of living crisis deepens across the continent", "ECONOMY"},

	// BUSINESS
	{"Tech giant reports record quarterly earnings", "BUSINESS"},
	{"Startup raises fifty million in Series B funding round", "BUSINESS"},
	{"Carmaker announces merger with smaller rival", "BUSINESS"},
	{"Retailer cuts five thousand jobs in restructuring", "BUSINESS"},
	{"Chief executive steps down after a decade at the helm", "BUSINESS"},
	{"Conglomerate acquires competitor in two billion buyout", "BUSINESS"},
	{"Payments company files for IPO on the exchange", "BUSINESS"},
	{"Quarterly profit misses estimates as guidance is cut", "BUSINESS"},
	{"Airline appoints new chief executive to lead turnaround", "BUSINESS"},
	{"Manufacturer unveils sweeping restructuring plan", "BUSINESS"},

	// TECHNOLOGY
	{"OpenAI releases a new AI model with reasoning skills", "TECHNOLOGY"},
	{"Ransomware attack cripples a major logistics firm", "TECHNOLOGY"},
	{"Phone maker debuts flagship smartphone at product launch", "TECHNOLOGY"},
	{"Social media platform rolls out content moderation rules", "TECHNOLOGY"},
	{"Data breach exposes millions of user records", "TECHNOLOGY"},
	{"Google launches an AI chatbot to rival competitors", "TECHNOLOGY"},
	{"New malware strain targets Android devices", "TECHNOLOGY"},
	{"Chipmaker launches a neural network accelerator", "TECHNOLOGY"},
	{"Meta faces backlash over its Instagram algorithm", "TECHNOLOGY"},
	{"Startup ships a software update fixing the security flaw", "TECHNOLOGY"},

	// SCIENCE
	{"NASA launches a rocket on a mission to Mars", "SCIENCE"},
	{"Scientists discover a new species in the deep ocean", "SCIENCE"},
	{"Telescope captures a stunning image of a distant galaxy", "SCIENCE"},
	{"Study finds a link between diet and longevity", "SCIENCE"},
	{"SpaceX rocket reaches orbit on a test flight", "SCIENCE"},
	{"Researchers achieve a breakthrough in quantum physics", "SCIENCE"},
	{"Probe detects water on a nearby asteroid", "SCIENCE"},
	{"New fossil discovery rewrites the human timeline", "SCIENCE"},

	// ENVIRONMENT
	{"Global warming pushes temperatures to record highs", "ENVIRONMENT"},
	{"Air pollution chokes major cities through winter", "ENVIRONMENT"},
	{"Deforestation threatens endangered wildlife", "ENVIRONMENT"},
	{"Nations pledge net zero carbon emissions by 2050", "ENVIRONMENT"},
	{"Oil spill devastates a fragile coastal ecosystem", "ENVIRONMENT"},
	{"Climate crisis fuels more extreme weather", "ENVIRONMENT"},
	{"Conservationists race to save the coral reef", "ENVIRONMENT"},
	{"Plastic waste pollutes remote beaches", "ENVIRONMENT"},

	// DISASTER
	{"Powerful earthquake strikes a coastal region", "DISASTER"},
	{"Floods displace thousands after torrential rain", "DISASTER"},
	{"Hurricane makes landfall as a category four storm", "DISASTER"},
	{"Wildfire spreads rapidly through dry forest", "DISASTER"},
	{"Volcano erupts, forcing a mass evacuation", "DISASTER"},
	{"Landslide buries a village after heavy monsoon", "DISASTER"},
	{"Drought deepens as reservoirs run dry", "DISASTER"},
	{"Tsunami warning issued after an offshore tremor", "DISASTER"},

	// PUBLIC_HEALTH
	{"Measles outbreak spreads across several districts", "PUBLIC_HEALTH"},
	{"Regulators approve a new vaccine after clinical trials", "PUBLIC_HEALTH"},
	{"Hospitals strained as a flu epidemic worsens", "PUBLIC_HEALTH"},
	{"Health officials warn of a rising infection rate", "PUBLIC_HEALTH"},
	{"New drug treatment shows promise against the disease", "PUBLIC_HEALTH"},
	{"Pandemic preparedness plan unveiled by the ministry", "PUBLIC_HEALTH"},

	// LEGAL
	{"Supreme court rules against the sweeping mandate", "LEGAL"},
	{"Regulator fines the bank for compliance failures", "LEGAL"},
	{"Jury delivers a guilty verdict in the fraud trial", "LEGAL"},
	{"Antitrust watchdog opens an investigation into the merger", "LEGAL"},
	{"Company settles a lawsuit over defective products", "LEGAL"},
	{"Judge sentences the defendant to ten years", "LEGAL"},

	// CRIME
	{"Police arrest a suspect in the homicide case", "CRIME"},
	{"Massive fraud scheme uncovered at the brokerage", "CRIME"},
	{"Authorities bust a drug trafficking cartel", "CRIME"},
	{"Armed robbery at the jewelry store under investigation", "CRIME"},
	{"Man charged with assault after a violent altercation", "CRIME"},
	{"Investigators dismantle a money laundering ring", "CRIME"},

	// CONFLICT
	{"Airstrikes intensify along the contested frontline", "CONFLICT"},
	{"Suicide bombing kills dozens at a crowded market", "CONFLICT"},
	{"Navy deploys warships amid rising tensions", "CONFLICT"},
	{"Ceasefire collapses as shelling resumes", "CONFLICT"},
	{"Militants seize a town after fierce combat", "CONFLICT"},
	{"Defense ministry unveils a new missile system", "CONFLICT"},

	// SOCIETY
	{"Migrants stranded at the border seek asylum", "SOCIETY"},
	{"Human rights groups condemn the crackdown", "SOCIETY"},
	{"Workers strike as the union demands higher wages", "SOCIETY"},
	{"Pope leads a mass pilgrimage at the vatican", "SOCIETY"},
	{"Report highlights deepening poverty and inequality", "SOCIETY"},
	{"Refugees flee across the border amid the crisis", "SOCIETY"},

	// CULTURE
	{"Blockbuster film smashes box office records", "CULTURE"},
	{"Pop star announces a sold out world concert tour", "CULTURE"},
	{"Museum unveils a landmark art exhibition", "CULTURE"},
	{"Streaming series wins critical acclaim at the premiere", "CULTURE"},
	{"Best selling author releases a long awaited novel", "CULTURE"},
	{"Grammy winning singer drops a surprise album", "CULTURE"},

	// SPORTS
	{"Underdogs win the championship final in dramatic fashion", "SPORTS"},
	{"Club signs a striker in a record transfer deal", "SPORTS"},
	{"Home team defeats rivals to reach the playoffs", "SPORTS"},
	{"Athlete claims gold medal at the olympics", "SPORTS"},
	{"Cricket captain leads the team to a series victory", "SPORTS"},
	{"Football coach sacked after a losing streak", "SPORTS"},

	// EDUCATION
	{"Schools reopen as teachers end their walkout", "EDUCATION"},
	{"University raises tuition amid a funding shortfall", "EDUCATION"},
	{"Students protest cuts to the school curriculum", "EDUCATION"},
	{"College campus adopts new admissions rules", "EDUCATION"},
	{"Exam results reveal a widening attainment gap", "EDUCATION"},
	{"Edtech startup partners with universities", "EDUCATION"},

	// ENERGY
	{"Oil prices spike as OPEC cuts crude output", "ENERGY"},
	{"Solar and wind power capacity hits a new high", "ENERGY"},
	{"Nuclear reactor comes back online after maintenance", "ENERGY"},
	{"Power grid strained during a record heat wave", "ENERGY"},
	{"Natural gas pipeline project wins approval", "ENERGY"},
	{"Utility invests in battery storage for renewables", "ENERGY"},

	// TRANSPORT
	{"Airline grounds flights after a technical fault", "TRANSPORT"},
	{"Train derailment disrupts the rail network", "TRANSPORT"},
	{"Highway pileup causes major traffic delays", "TRANSPORT"},
	{"Cargo ship blocks a busy shipping port", "TRANSPORT"},
	{"Airport reopens runway after emergency repairs", "TRANSPORT"},
	{"Metro extension promises to ease commuter congestion", "TRANSPORT"},

	// Harder / adversarial cases — real headlines that exposed keyword gaps and
	// false positives during development (e.g. "European Union" matching a labor
	// keyword, "wins" hijacking a culture story). Kept as regressions.
	{"Typhoon forces evacuation of coastal towns in the Philippines", "DISASTER"},
	{"European Union unveils sweeping AI regulation", "LEGAL"},
	{"Striking nurses walk out over a pay dispute", "SOCIETY"},
	{"Archaeologists uncover an ancient tomb in Egypt", "SCIENCE"},
	{"Wildfire smoke blankets the city in haze", "DISASTER"},
	{"Central bank holds rates steady amid uncertainty", "ECONOMY"},
	{"Star quarterback traded to a rival franchise", "SPORTS"},
	{"Vatican announces the date for the papal conclave", "SOCIETY"},
	{"Cyberattack takes down government websites", "TECHNOLOGY"},
	{"Oil tanker runs aground, spilling crude into the bay", "ENERGY"},
	{"University students rally against tuition hikes", "EDUCATION"},
	{"Rebels launch an offensive near the capital", "CONFLICT"},
	{"Broadway musical wins top honors at an awards show", "CULTURE"},
}

func TestClassificationCorpusFloor(t *testing.T) {
	var general, correctDomain int
	var generalMisses, domainMisses []string

	for _, c := range classificationCorpus {
		tags := ClassifyText(c.headline, "")
		got := tags[0].Code
		if got == "GENERAL.OTHER" {
			general++
			generalMisses = append(generalMisses, c.headline)
		}
		if domainOf(got) == c.domain {
			correctDomain++
		} else {
			domainMisses = append(domainMisses, c.headline+"  => got "+got+" want "+c.domain)
		}
	}

	total := len(classificationCorpus)
	nonGeneralRate := float64(total-general) / float64(total)
	domainRate := float64(correctDomain) / float64(total)
	t.Logf("corpus size=%d  non-GENERAL=%.1f%%  domain-accuracy=%.1f%%",
		total, nonGeneralRate*100, domainRate*100)

	// Primary goal: at most ~1% may fall through to GENERAL.OTHER.
	if nonGeneralRate < 0.99 {
		t.Errorf("non-GENERAL rate %.1f%% below 99%% floor; fell through to GENERAL.OTHER:\n  %s",
			nonGeneralRate*100, strings.Join(generalMisses, "\n  "))
	}
	// Secondary goal: the assigned domain should be correct for the vast majority.
	if domainRate < 0.90 {
		t.Errorf("domain accuracy %.1f%% below 90%% floor; mismatches:\n  %s",
			domainRate*100, strings.Join(domainMisses, "\n  "))
	}
	if len(domainMisses) > 0 {
		t.Logf("domain mismatches (%d):\n  %s", len(domainMisses), strings.Join(domainMisses, "\n  "))
	}
}
