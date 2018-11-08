package benchmark

/*
   Small paylod, http log like structure. Size: 190 bytes
*/
var smallFixture []byte = []byte(`{
    "st": 1,
    "sid": 486,
    "tt": "active",
    "gr": 0,
    "uuid": "de305d54-75b4-431b-adb2-eb6b9e546014",
    "ip": "127.0.0.1",
    "ua": "user_agent",
    "tz": -6,
    "v": 1
}`)

type SmallPayload struct {
	St   int
	Sid  int
	Tt   string
	Gr   int
	Uuid string
	Ip   string
	Ua   string
	Tz   int
	V    int
}

/*
   Medium payload (based on Clearbit API response)
*/
type CBAvatar struct {
	Url string
}

type CBGravatar struct {
	Avatars []*CBAvatar
}

type CBGithub struct {
	Followers int
}

type CBName struct {
	FullName string
}

type CBPerson struct {
	Name     *CBName
	Github   *CBGithub
	Gravatar *CBGravatar
}

type MediumPayload struct {
	Person  *CBPerson
	Company map[string]interface{}
}

// Reponse from Clearbit API. Size: 2.4kb
var mediumFixture []byte = []byte(`{
  "person": {
    "id": "d50887ca-a6ce-4e59-b89f-14f0b5d03b03",
    "name": {
      "fullName": "Leonid Bugaev",
      "givenName": "Leonid",
      "familyName": "Bugaev"
    },
    "email": "leonsbox@gmail.com",
    "gender": "male",
    "location": "Saint Petersburg, Saint Petersburg, RU",
    "geo": {
      "city": "Saint Petersburg",
      "state": "Saint Petersburg",
      "country": "Russia",
      "lat": 59.9342802,
      "lng": 30.3350986
    },
    "bio": "Senior engineer at Granify.com",
    "site": "http://flickfaver.com",
    "avatar": "https://d1ts43dypk8bqh.cloudfront.net/v1/avatars/d50887ca-a6ce-4e59-b89f-14f0b5d03b03",
    "employment": {
      "name": "www.latera.ru",
      "title": "Software Engineer",
      "domain": "gmail.com"
    },
    "facebook": {
      "handle": "leonid.bugaev"
    },
    "github": {
      "handle": "buger",
      "id": 14009,
      "avatar": "https://avatars.githubusercontent.com/u/14009?v=3",
      "company": "Granify",
      "blog": "http://leonsbox.com",
      "followers": 95,
      "following": 10
    },
    "twitter": {
      "handle": "flickfaver",
      "id": 77004410,
      "bio": null,
      "followers": 2,
      "following": 1,
      "statuses": 5,
      "favorites": 0,
      "location": "",
      "site": "http://flickfaver.com",
      "avatar": null
    },
    "linkedin": {
      "handle": "in/leonidbugaev"
    },
    "googleplus": {
      "handle": null
    },
    "angellist": {
      "handle": "leonid-bugaev",
      "id": 61541,
      "bio": "Senior engineer at Granify.com",
      "blog": "http://buger.github.com",
      "site": "http://buger.github.com",
      "followers": 41,
      "avatar": "https://d1qb2nb5cznatu.cloudfront.net/users/61541-medium_jpg?1405474390"
    },
    "klout": {
      "handle": null,
      "score": null
    },
    "foursquare": {
      "handle": null
    },
    "aboutme": {
      "handle": "leonid.bugaev",
      "bio": null,
      "avatar": null
    },
    "gravatar": {
      "handle": "buger",
      "urls": [

      ],
      "avatar": "http://1.gravatar.com/avatar/f7c8edd577d13b8930d5522f28123510",
      "avatars": [
        {
          "url": "http://1.gravatar.com/avatar/f7c8edd577d13b8930d5522f28123510",
          "type": "thumbnail"
        }
      ]
    },
    "fuzzy": false
  },
  "company": null
}`)

/*
   Large payload, based on Discourse API. Size: 28kb
*/

type DSUser struct {
	Username string
}

type DSTopic struct {
	Id   int
	Slug string
}

type DSTopicsList struct {
	Topics        []*DSTopic
	MoreTopicsUrl string
}

type LargePayload struct {
	Users  []*DSUser
	Topics *DSTopicsList
}

var largeFixture []byte = []byte(`
    {"users":[{"id":-1,"username":"system","avatar_template":"/user_avatar/discourse.metabase.com/system/{size}/6_1.png"},{"id":89,"username":"zergot","avatar_template":"https://avatars.discourse.org/v2/letter/z/0ea827/{size}.png"},{"id":1,"username":"sameer","avatar_template":"https://avatars.discourse.org/v2/letter/s/bbce88/{size}.png"},{"id":84,"username":"HenryMirror","avatar_template":"https://avatars.discourse.org/v2/letter/h/ecd19e/{size}.png"},{"id":73,"username":"fimp","avatar_template":"https://avatars.discourse.org/v2/letter/f/ee59a6/{size}.png"},{"id":14,"username":"agilliland","avatar_template":"/user_avatar/discourse.metabase.com/agilliland/{size}/26_1.png"},{"id":87,"username":"amir","avatar_template":"https://avatars.discourse.org/v2/letter/a/c37758/{size}.png"},{"id":82,"username":"waseem","avatar_template":"https://avatars.discourse.org/v2/letter/w/9dc877/{size}.png"},{"id":78,"username":"tovenaar","avatar_template":"https://avatars.discourse.org/v2/letter/t/9de0a6/{size}.png"},{"id":74,"username":"Ben","avatar_template":"https://avatars.discourse.org/v2/letter/b/df788c/{size}.png"},{"id":71,"username":"MarkLaFay","avatar_template":"https://avatars.discourse.org/v2/letter/m/3bc359/{size}.png"},{"id":72,"username":"camsaul","avatar_template":"/user_avatar/discourse.metabase.com/camsaul/{size}/70_1.png"},{"id":53,"username":"mhjb","avatar_template":"/user_avatar/discourse.metabase.com/mhjb/{size}/54_1.png"},{"id":58,"username":"jbwiv","avatar_template":"https://avatars.discourse.org/v2/letter/j/6bbea6/{size}.png"},{"id":70,"username":"Maggs","avatar_template":"https://avatars.discourse.org/v2/letter/m/bbce88/{size}.png"},{"id":69,"username":"andrefaria","avatar_template":"/user_avatar/discourse.metabase.com/andrefaria/{size}/65_1.png"},{"id":60,"username":"bencarter78","avatar_template":"/user_avatar/discourse.metabase.com/bencarter78/{size}/59_1.png"},{"id":55,"username":"vikram","avatar_template":"https://avatars.discourse.org/v2/letter/v/e47774/{size}.png"},{"id":68,"username":"edchan77","avatar_template":"/user_avatar/discourse.metabase.com/edchan77/{size}/66_1.png"},{"id":9,"username":"karthikd","avatar_template":"https://avatars.discourse.org/v2/letter/k/cab0a1/{size}.png"},{"id":23,"username":"arthurz","avatar_template":"/user_avatar/discourse.metabase.com/arthurz/{size}/32_1.png"},{"id":3,"username":"tom","avatar_template":"/user_avatar/discourse.metabase.com/tom/{size}/21_1.png"},{"id":50,"username":"LeoNogueira","avatar_template":"/user_avatar/discourse.metabase.com/leonogueira/{size}/52_1.png"},{"id":66,"username":"ss06vi","avatar_template":"https://avatars.discourse.org/v2/letter/s/3ab097/{size}.png"},{"id":34,"username":"mattcollins","avatar_template":"/user_avatar/discourse.metabase.com/mattcollins/{size}/41_1.png"},{"id":51,"username":"krmmalik","avatar_template":"/user_avatar/discourse.metabase.com/krmmalik/{size}/53_1.png"},{"id":46,"username":"odysseas","avatar_template":"https://avatars.discourse.org/v2/letter/o/5f8ce5/{size}.png"},{"id":5,"username":"jonthewayne","avatar_template":"/user_avatar/discourse.metabase.com/jonthewayne/{size}/18_1.png"},{"id":11,"username":"anandiyer","avatar_template":"/user_avatar/discourse.metabase.com/anandiyer/{size}/23_1.png"},{"id":25,"username":"alnorth","avatar_template":"/user_avatar/discourse.metabase.com/alnorth/{size}/34_1.png"},{"id":52,"username":"j_at_svg","avatar_template":"https://avatars.discourse.org/v2/letter/j/96bed5/{size}.png"},{"id":42,"username":"styts","avatar_template":"/user_avatar/discourse.metabase.com/styts/{size}/47_1.png"}],"topics":{"can_create_topic":false,"more_topics_url":"/c/uncategorized/l/latest?page=1","draft":null,"draft_key":"new_topic","draft_sequence":null,"per_page":30,"topics":[{"id":8,"title":"Welcome to Metabase's Discussion Forum","fancy_title":"Welcome to Metabase&rsquo;s Discussion Forum","slug":"welcome-to-metabases-discussion-forum","posts_count":1,"reply_count":0,"highest_post_number":1,"image_url":"/images/welcome/discourse-edit-post-animated.gif","created_at":"2015-10-17T00:14:49.526Z","last_posted_at":"2015-10-17T00:14:49.557Z","bumped":true,"bumped_at":"2015-10-21T02:32:22.486Z","unseen":false,"pinned":true,"unpinned":null,"excerpt":"Welcome to Metabase&#39;s discussion forum. This is a place to get help on installation, setting up as well as sharing tips and tricks.","visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":197,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"system","category_id":1,"pinned_globally":true,"posters":[{"extras":"latest single","description":"Original Poster, Most Recent Poster","user_id":-1}]},{"id":169,"title":"Formatting Dates","fancy_title":"Formatting Dates","slug":"formatting-dates","posts_count":1,"reply_count":0,"highest_post_number":1,"image_url":null,"created_at":"2016-01-14T06:30:45.311Z","last_posted_at":"2016-01-14T06:30:45.397Z","bumped":true,"bumped_at":"2016-01-14T06:30:45.397Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":11,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"zergot","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest single","description":"Original Poster, Most Recent Poster","user_id":89}]},{"id":168,"title":"Setting for google api key","fancy_title":"Setting for google api key","slug":"setting-for-google-api-key","posts_count":2,"reply_count":0,"highest_post_number":2,"image_url":null,"created_at":"2016-01-13T17:14:31.799Z","last_posted_at":"2016-01-14T06:24:03.421Z","bumped":true,"bumped_at":"2016-01-14T06:24:03.421Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":16,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"zergot","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest single","description":"Original Poster, Most Recent Poster","user_id":89}]},{"id":167,"title":"Cannot see non-US timezones on the admin","fancy_title":"Cannot see non-US timezones on the admin","slug":"cannot-see-non-us-timezones-on-the-admin","posts_count":1,"reply_count":0,"highest_post_number":1,"image_url":null,"created_at":"2016-01-13T17:07:36.764Z","last_posted_at":"2016-01-13T17:07:36.831Z","bumped":true,"bumped_at":"2016-01-13T17:07:36.831Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":11,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"zergot","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest single","description":"Original Poster, Most Recent Poster","user_id":89}]},{"id":164,"title":"External (Metabase level) linkages in data schema","fancy_title":"External (Metabase level) linkages in data schema","slug":"external-metabase-level-linkages-in-data-schema","posts_count":4,"reply_count":1,"highest_post_number":4,"image_url":null,"created_at":"2016-01-11T13:51:02.286Z","last_posted_at":"2016-01-12T11:06:37.259Z","bumped":true,"bumped_at":"2016-01-12T11:06:37.259Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":32,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"zergot","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest","description":"Original Poster, Most Recent Poster","user_id":89},{"extras":null,"description":"Frequent Poster","user_id":1}]},{"id":155,"title":"Query working on \"Questions\" but not in \"Pulses\"","fancy_title":"Query working on &ldquo;Questions&rdquo; but not in &ldquo;Pulses&rdquo;","slug":"query-working-on-questions-but-not-in-pulses","posts_count":3,"reply_count":0,"highest_post_number":3,"image_url":null,"created_at":"2016-01-01T14:06:10.083Z","last_posted_at":"2016-01-08T22:37:51.772Z","bumped":true,"bumped_at":"2016-01-08T22:37:51.772Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":72,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"agilliland","category_id":1,"pinned_globally":false,"posters":[{"extras":null,"description":"Original Poster","user_id":84},{"extras":null,"description":"Frequent Poster","user_id":73},{"extras":"latest","description":"Most Recent Poster","user_id":14}]},{"id":161,"title":"Pulses posted to Slack don't show question output","fancy_title":"Pulses posted to Slack don&rsquo;t show question output","slug":"pulses-posted-to-slack-dont-show-question-output","posts_count":2,"reply_count":0,"highest_post_number":2,"image_url":"/uploads/default/original/1X/9d2806517bf3598b10be135b2c58923b47ba23e7.png","created_at":"2016-01-08T22:09:58.205Z","last_posted_at":"2016-01-08T22:28:44.685Z","bumped":true,"bumped_at":"2016-01-08T22:28:44.685Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":34,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"sameer","category_id":1,"pinned_globally":false,"posters":[{"extras":null,"description":"Original Poster","user_id":87},{"extras":"latest","description":"Most Recent Poster","user_id":1}]},{"id":152,"title":"Should we build Kafka connecter or Kafka plugin","fancy_title":"Should we build Kafka connecter or Kafka plugin","slug":"should-we-build-kafka-connecter-or-kafka-plugin","posts_count":4,"reply_count":1,"highest_post_number":4,"image_url":null,"created_at":"2015-12-28T20:37:23.501Z","last_posted_at":"2015-12-31T18:16:45.477Z","bumped":true,"bumped_at":"2015-12-31T18:16:45.477Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":84,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"sameer","category_id":1,"pinned_globally":false,"posters":[{"extras":null,"description":"Original Poster","user_id":82},{"extras":"latest","description":"Most Recent Poster, Frequent Poster","user_id":1}]},{"id":147,"title":"Change X and Y on graph","fancy_title":"Change X and Y on graph","slug":"change-x-and-y-on-graph","posts_count":1,"reply_count":0,"highest_post_number":1,"image_url":null,"created_at":"2015-12-21T17:52:46.581Z","last_posted_at":"2015-12-21T17:52:46.684Z","bumped":true,"bumped_at":"2015-12-21T18:19:13.003Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":68,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"tovenaar","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest single","description":"Original Poster, Most Recent Poster","user_id":78}]},{"id":142,"title":"Issues sending mail via office365 relay","fancy_title":"Issues sending mail via office365 relay","slug":"issues-sending-mail-via-office365-relay","posts_count":5,"reply_count":2,"highest_post_number":5,"image_url":null,"created_at":"2015-12-16T10:38:47.315Z","last_posted_at":"2015-12-21T09:26:27.167Z","bumped":true,"bumped_at":"2015-12-21T09:26:27.167Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":122,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"Ben","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest","description":"Original Poster, Most Recent Poster","user_id":74},{"extras":null,"description":"Frequent Poster","user_id":1}]},{"id":137,"title":"I see triplicates of my mongoDB collections","fancy_title":"I see triplicates of my mongoDB collections","slug":"i-see-triplicates-of-my-mongodb-collections","posts_count":3,"reply_count":0,"highest_post_number":3,"image_url":null,"created_at":"2015-12-14T13:33:03.426Z","last_posted_at":"2015-12-17T18:40:05.487Z","bumped":true,"bumped_at":"2015-12-17T18:40:05.487Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":97,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"MarkLaFay","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest","description":"Original Poster, Most Recent Poster","user_id":71},{"extras":null,"description":"Frequent Poster","user_id":14}]},{"id":140,"title":"Google Analytics plugin","fancy_title":"Google Analytics plugin","slug":"google-analytics-plugin","posts_count":1,"reply_count":0,"highest_post_number":1,"image_url":null,"created_at":"2015-12-15T13:00:55.644Z","last_posted_at":"2015-12-15T13:00:55.705Z","bumped":true,"bumped_at":"2015-12-15T13:00:55.705Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":105,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"fimp","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest single","description":"Original Poster, Most Recent Poster","user_id":73}]},{"id":138,"title":"With-mongo-connection failed: bad connection details:","fancy_title":"With-mongo-connection failed: bad connection details:","slug":"with-mongo-connection-failed-bad-connection-details","posts_count":1,"reply_count":0,"highest_post_number":1,"image_url":null,"created_at":"2015-12-14T17:28:11.041Z","last_posted_at":"2015-12-14T17:28:11.111Z","bumped":true,"bumped_at":"2015-12-14T17:28:11.111Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":56,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"MarkLaFay","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest single","description":"Original Poster, Most Recent Poster","user_id":71}]},{"id":133,"title":"\"We couldn't understand your question.\" when I query mongoDB","fancy_title":"&ldquo;We couldn&rsquo;t understand your question.&rdquo; when I query mongoDB","slug":"we-couldnt-understand-your-question-when-i-query-mongodb","posts_count":3,"reply_count":0,"highest_post_number":3,"image_url":null,"created_at":"2015-12-11T17:38:30.576Z","last_posted_at":"2015-12-14T13:31:26.395Z","bumped":true,"bumped_at":"2015-12-14T13:31:26.395Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":107,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"MarkLaFay","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest","description":"Original Poster, Most Recent Poster","user_id":71},{"extras":null,"description":"Frequent Poster","user_id":72}]},{"id":129,"title":"My bar charts are all thin","fancy_title":"My bar charts are all thin","slug":"my-bar-charts-are-all-thin","posts_count":4,"reply_count":1,"highest_post_number":4,"image_url":"/uploads/default/original/1X/41bcf3b2a00dc7cfaff01cb3165d35d32a85bf1d.png","created_at":"2015-12-09T22:09:56.394Z","last_posted_at":"2015-12-11T19:00:45.289Z","bumped":true,"bumped_at":"2015-12-11T19:00:45.289Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":116,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"mhjb","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest","description":"Original Poster, Most Recent Poster","user_id":53},{"extras":null,"description":"Frequent Poster","user_id":1}]},{"id":106,"title":"What is the expected return order of columns for graphing results when using raw SQL?","fancy_title":"What is the expected return order of columns for graphing results when using raw SQL?","slug":"what-is-the-expected-return-order-of-columns-for-graphing-results-when-using-raw-sql","posts_count":3,"reply_count":0,"highest_post_number":3,"image_url":null,"created_at":"2015-11-24T19:07:14.561Z","last_posted_at":"2015-12-11T17:04:14.149Z","bumped":true,"bumped_at":"2015-12-11T17:04:14.149Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":153,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"jbwiv","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest","description":"Original Poster, Most Recent Poster","user_id":58},{"extras":null,"description":"Frequent Poster","user_id":14}]},{"id":131,"title":"Set site url from admin panel","fancy_title":"Set site url from admin panel","slug":"set-site-url-from-admin-panel","posts_count":2,"reply_count":0,"highest_post_number":2,"image_url":null,"created_at":"2015-12-10T06:22:46.042Z","last_posted_at":"2015-12-10T19:12:57.449Z","bumped":true,"bumped_at":"2015-12-10T19:12:57.449Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":77,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"sameer","category_id":1,"pinned_globally":false,"posters":[{"extras":null,"description":"Original Poster","user_id":70},{"extras":"latest","description":"Most Recent Poster","user_id":1}]},{"id":127,"title":"Internationalization (i18n)","fancy_title":"Internationalization (i18n)","slug":"internationalization-i18n","posts_count":2,"reply_count":0,"highest_post_number":2,"image_url":null,"created_at":"2015-12-08T16:55:37.397Z","last_posted_at":"2015-12-09T16:49:55.816Z","bumped":true,"bumped_at":"2015-12-09T16:49:55.816Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":85,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"agilliland","category_id":1,"pinned_globally":false,"posters":[{"extras":null,"description":"Original Poster","user_id":69},{"extras":"latest","description":"Most Recent Poster","user_id":14}]},{"id":109,"title":"Returning raw data with no filters always returns We couldn't understand your question","fancy_title":"Returning raw data with no filters always returns We couldn&rsquo;t understand your question","slug":"returning-raw-data-with-no-filters-always-returns-we-couldnt-understand-your-question","posts_count":3,"reply_count":1,"highest_post_number":3,"image_url":null,"created_at":"2015-11-25T21:35:01.315Z","last_posted_at":"2015-12-09T10:26:12.255Z","bumped":true,"bumped_at":"2015-12-09T10:26:12.255Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":133,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"bencarter78","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest","description":"Original Poster, Most Recent Poster","user_id":60},{"extras":null,"description":"Frequent Poster","user_id":14}]},{"id":103,"title":"Support for Cassandra?","fancy_title":"Support for Cassandra?","slug":"support-for-cassandra","posts_count":5,"reply_count":1,"highest_post_number":5,"image_url":null,"created_at":"2015-11-20T06:45:31.741Z","last_posted_at":"2015-12-09T03:18:51.274Z","bumped":true,"bumped_at":"2015-12-09T03:18:51.274Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":169,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"vikram","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest","description":"Original Poster, Most Recent Poster","user_id":55},{"extras":null,"description":"Frequent Poster","user_id":1}]},{"id":128,"title":"Mongo query with Date breaks [solved: Mongo 3.0 required]","fancy_title":"Mongo query with Date breaks [solved: Mongo 3.0 required]","slug":"mongo-query-with-date-breaks-solved-mongo-3-0-required","posts_count":5,"reply_count":0,"highest_post_number":5,"image_url":null,"created_at":"2015-12-08T18:30:56.562Z","last_posted_at":"2015-12-08T21:03:02.421Z","bumped":true,"bumped_at":"2015-12-08T21:03:02.421Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":102,"like_count":1,"has_summary":false,"archetype":"regular","last_poster_username":"edchan77","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest","description":"Original Poster, Most Recent Poster","user_id":68},{"extras":null,"description":"Frequent Poster","user_id":1}]},{"id":23,"title":"Can this connect to MS SQL Server?","fancy_title":"Can this connect to MS SQL Server?","slug":"can-this-connect-to-ms-sql-server","posts_count":7,"reply_count":1,"highest_post_number":7,"image_url":null,"created_at":"2015-10-21T18:52:37.987Z","last_posted_at":"2015-12-07T17:41:51.609Z","bumped":true,"bumped_at":"2015-12-07T17:41:51.609Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":367,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"sameer","category_id":1,"pinned_globally":false,"posters":[{"extras":null,"description":"Original Poster","user_id":9},{"extras":null,"description":"Frequent Poster","user_id":23},{"extras":null,"description":"Frequent Poster","user_id":3},{"extras":null,"description":"Frequent Poster","user_id":50},{"extras":"latest","description":"Most Recent Poster","user_id":1}]},{"id":121,"title":"Cannot restart metabase in docker","fancy_title":"Cannot restart metabase in docker","slug":"cannot-restart-metabase-in-docker","posts_count":5,"reply_count":1,"highest_post_number":5,"image_url":null,"created_at":"2015-12-04T21:28:58.137Z","last_posted_at":"2015-12-04T23:02:00.488Z","bumped":true,"bumped_at":"2015-12-04T23:02:00.488Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":96,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"sameer","category_id":1,"pinned_globally":false,"posters":[{"extras":null,"description":"Original Poster","user_id":66},{"extras":"latest","description":"Most Recent Poster, Frequent Poster","user_id":1}]},{"id":85,"title":"Edit Max Rows Count","fancy_title":"Edit Max Rows Count","slug":"edit-max-rows-count","posts_count":4,"reply_count":2,"highest_post_number":4,"image_url":null,"created_at":"2015-11-11T23:46:52.917Z","last_posted_at":"2015-11-24T01:01:14.569Z","bumped":true,"bumped_at":"2015-11-24T01:01:14.569Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":169,"like_count":1,"has_summary":false,"archetype":"regular","last_poster_username":"sameer","category_id":1,"pinned_globally":false,"posters":[{"extras":null,"description":"Original Poster","user_id":34},{"extras":"latest","description":"Most Recent Poster, Frequent Poster","user_id":1}]},{"id":96,"title":"Creating charts by querying more than one table at a time","fancy_title":"Creating charts by querying more than one table at a time","slug":"creating-charts-by-querying-more-than-one-table-at-a-time","posts_count":6,"reply_count":4,"highest_post_number":6,"image_url":null,"created_at":"2015-11-17T11:20:18.442Z","last_posted_at":"2015-11-21T02:12:25.995Z","bumped":true,"bumped_at":"2015-11-21T02:12:25.995Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":217,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"sameer","category_id":1,"pinned_globally":false,"posters":[{"extras":null,"description":"Original Poster","user_id":51},{"extras":"latest","description":"Most Recent Poster, Frequent Poster","user_id":1}]},{"id":90,"title":"Trying to add RDS postgresql as the database fails silently","fancy_title":"Trying to add RDS postgresql as the database fails silently","slug":"trying-to-add-rds-postgresql-as-the-database-fails-silently","posts_count":4,"reply_count":2,"highest_post_number":4,"image_url":null,"created_at":"2015-11-14T23:45:02.967Z","last_posted_at":"2015-11-21T01:08:45.915Z","bumped":true,"bumped_at":"2015-11-21T01:08:45.915Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":162,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"sameer","category_id":1,"pinned_globally":false,"posters":[{"extras":null,"description":"Original Poster","user_id":46},{"extras":"latest","description":"Most Recent Poster, Frequent Poster","user_id":1}]},{"id":17,"title":"Deploy to Heroku isn't working","fancy_title":"Deploy to Heroku isn&rsquo;t working","slug":"deploy-to-heroku-isnt-working","posts_count":9,"reply_count":3,"highest_post_number":9,"image_url":null,"created_at":"2015-10-21T16:42:03.096Z","last_posted_at":"2015-11-20T18:34:14.044Z","bumped":true,"bumped_at":"2015-11-20T18:34:14.044Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":332,"like_count":2,"has_summary":false,"archetype":"regular","last_poster_username":"agilliland","category_id":1,"pinned_globally":false,"posters":[{"extras":null,"description":"Original Poster","user_id":5},{"extras":null,"description":"Frequent Poster","user_id":3},{"extras":null,"description":"Frequent Poster","user_id":11},{"extras":null,"description":"Frequent Poster","user_id":25},{"extras":"latest","description":"Most Recent Poster","user_id":14}]},{"id":100,"title":"Can I use DATEPART() in SQL queries?","fancy_title":"Can I use DATEPART() in SQL queries?","slug":"can-i-use-datepart-in-sql-queries","posts_count":2,"reply_count":0,"highest_post_number":2,"image_url":null,"created_at":"2015-11-17T23:15:58.033Z","last_posted_at":"2015-11-18T00:19:48.763Z","bumped":true,"bumped_at":"2015-11-18T00:19:48.763Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":112,"like_count":1,"has_summary":false,"archetype":"regular","last_poster_username":"sameer","category_id":1,"pinned_globally":false,"posters":[{"extras":null,"description":"Original Poster","user_id":53},{"extras":"latest","description":"Most Recent Poster","user_id":1}]},{"id":98,"title":"Feature Request: LDAP Authentication","fancy_title":"Feature Request: LDAP Authentication","slug":"feature-request-ldap-authentication","posts_count":1,"reply_count":0,"highest_post_number":1,"image_url":null,"created_at":"2015-11-17T17:22:44.484Z","last_posted_at":"2015-11-17T17:22:44.577Z","bumped":true,"bumped_at":"2015-11-17T17:22:44.577Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":97,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"j_at_svg","category_id":1,"pinned_globally":false,"posters":[{"extras":"latest single","description":"Original Poster, Most Recent Poster","user_id":52}]},{"id":87,"title":"Migrating from internal H2 to Postgres","fancy_title":"Migrating from internal H2 to Postgres","slug":"migrating-from-internal-h2-to-postgres","posts_count":2,"reply_count":0,"highest_post_number":2,"image_url":null,"created_at":"2015-11-12T14:36:06.745Z","last_posted_at":"2015-11-12T18:05:10.796Z","bumped":true,"bumped_at":"2015-11-12T18:05:10.796Z","unseen":false,"pinned":false,"unpinned":null,"visible":true,"closed":false,"archived":false,"bookmarked":null,"liked":null,"views":111,"like_count":0,"has_summary":false,"archetype":"regular","last_poster_username":"sameer","category_id":1,"pinned_globally":false,"posters":[{"extras":null,"description":"Original Poster","user_id":42},{"extras":"latest","description":"Most Recent Poster","user_id":1}]}]}}
`)
