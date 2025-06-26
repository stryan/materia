aliases:
  - "mailto:bob@foobar.com"
  - "https://mastodon/bob"
properties:
  'http://webfinger.example/ns/name': 'Bob Smith'
links:
  - rel: "http://webfinger.example/rel/profile-page"
    href: "https://www.example.com/~bob/"
  - rel: "http://webfinger.example/rel/businesscard"
    href: "https://www.example.com/~bob/bob.vcf"
