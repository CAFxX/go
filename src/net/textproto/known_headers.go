// Code generated by genheader.go. DO NOT EDIT.
package textproto

// headerToString matches the canonical header name in the passed slice
// and returns the singleton string instance of known, common headers.
// If the header is not known, the passed slice is returned as a string.
//
// A header is eligible for inclusion in headerToString if, either:
//  1. It is a standard header, e.g. listed in a non-obsolete RFC or other
//     standards.
//  2. It is a header that can be shown to be commonly used in the wild.
func headerToString(a []byte) (s string) {
	switch string(a) { // Compiler knows how to avoid allocating the string.
	case "A-Im":
		s = "A-Im"
	case "Accept":
		s = "Accept"
	case "Accept-Additions":
		s = "Accept-Additions"
	case "Accept-Ch":
		s = "Accept-Ch"
	case "Accept-Charset":
		s = "Accept-Charset"
	case "Accept-Datetime":
		s = "Accept-Datetime"
	case "Accept-Encoding":
		s = "Accept-Encoding"
	case "Accept-Features":
		s = "Accept-Features"
	case "Accept-Language":
		s = "Accept-Language"
	case "Accept-Patch":
		s = "Accept-Patch"
	case "Accept-Post":
		s = "Accept-Post"
	case "Accept-Ranges":
		s = "Accept-Ranges"
	case "Access-Control":
		s = "Access-Control"
	case "Access-Control-Allow-Credentials":
		s = "Access-Control-Allow-Credentials"
	case "Access-Control-Allow-Headers":
		s = "Access-Control-Allow-Headers"
	case "Access-Control-Allow-Methods":
		s = "Access-Control-Allow-Methods"
	case "Access-Control-Allow-Origin":
		s = "Access-Control-Allow-Origin"
	case "Access-Control-Expose-Headers":
		s = "Access-Control-Expose-Headers"
	case "Access-Control-Max-Age":
		s = "Access-Control-Max-Age"
	case "Access-Control-Request-Headers":
		s = "Access-Control-Request-Headers"
	case "Access-Control-Request-Method":
		s = "Access-Control-Request-Method"
	case "Age":
		s = "Age"
	case "Allow":
		s = "Allow"
	case "Alpn":
		s = "Alpn"
	case "Also-Control":
		s = "Also-Control"
	case "Alt-Svc":
		s = "Alt-Svc"
	case "Alt-Used":
		s = "Alt-Used"
	case "Alternate-Recipient":
		s = "Alternate-Recipient"
	case "Alternates":
		s = "Alternates"
	case "Amp-Cache-Transform":
		s = "Amp-Cache-Transform"
	case "Apply-To-Redirect-Ref":
		s = "Apply-To-Redirect-Ref"
	case "Approved":
		s = "Approved"
	case "Arc-Authentication-Results":
		s = "Arc-Authentication-Results"
	case "Arc-Message-Signature":
		s = "Arc-Message-Signature"
	case "Arc-Seal":
		s = "Arc-Seal"
	case "Archive":
		s = "Archive"
	case "Archived-At":
		s = "Archived-At"
	case "Article-Names":
		s = "Article-Names"
	case "Article-Updates":
		s = "Article-Updates"
	case "Authentication-Control":
		s = "Authentication-Control"
	case "Authentication-Info":
		s = "Authentication-Info"
	case "Authentication-Results":
		s = "Authentication-Results"
	case "Authorization":
		s = "Authorization"
	case "Auto-Submitted":
		s = "Auto-Submitted"
	case "Autoforwarded":
		s = "Autoforwarded"
	case "Autosubmitted":
		s = "Autosubmitted"
	case "Base":
		s = "Base"
	case "Bcc":
		s = "Bcc"
	case "Body":
		s = "Body"
	case "C-Ext":
		s = "C-Ext"
	case "C-Man":
		s = "C-Man"
	case "C-Opt":
		s = "C-Opt"
	case "C-Pep":
		s = "C-Pep"
	case "C-Pep-Info":
		s = "C-Pep-Info"
	case "Cache-Control":
		s = "Cache-Control"
	case "Cache-Status":
		s = "Cache-Status"
	case "Cal-Managed-Id":
		s = "Cal-Managed-Id"
	case "Caldav-Timezones":
		s = "Caldav-Timezones"
	case "Cancel-Key":
		s = "Cancel-Key"
	case "Cancel-Lock":
		s = "Cancel-Lock"
	case "Capsule-Protocol":
		s = "Capsule-Protocol"
	case "Cc":
		s = "Cc"
	case "Cdn-Cache-Control":
		s = "Cdn-Cache-Control"
	case "Cdn-Loop":
		s = "Cdn-Loop"
	case "Cert-Not-After":
		s = "Cert-Not-After"
	case "Cert-Not-Before":
		s = "Cert-Not-Before"
	case "Clear-Site-Data":
		s = "Clear-Site-Data"
	case "Client-Cert":
		s = "Client-Cert"
	case "Client-Cert-Chain":
		s = "Client-Cert-Chain"
	case "Close":
		s = "Close"
	case "Comments":
		s = "Comments"
	case "Configuration-Context":
		s = "Configuration-Context"
	case "Connection":
		s = "Connection"
	case "Content-Alternative":
		s = "Content-Alternative"
	case "Content-Base":
		s = "Content-Base"
	case "Content-Description":
		s = "Content-Description"
	case "Content-Disposition":
		s = "Content-Disposition"
	case "Content-Duration":
		s = "Content-Duration"
	case "Content-Encoding":
		s = "Content-Encoding"
	case "Content-Features":
		s = "Content-Features"
	case "Content-Id":
		s = "Content-Id"
	case "Content-Identifier":
		s = "Content-Identifier"
	case "Content-Language":
		s = "Content-Language"
	case "Content-Length":
		s = "Content-Length"
	case "Content-Location":
		s = "Content-Location"
	case "Content-Md5":
		s = "Content-Md5"
	case "Content-Range":
		s = "Content-Range"
	case "Content-Return":
		s = "Content-Return"
	case "Content-Script-Type":
		s = "Content-Script-Type"
	case "Content-Security-Policy":
		s = "Content-Security-Policy"
	case "Content-Security-Policy-Report-Only":
		s = "Content-Security-Policy-Report-Only"
	case "Content-Style-Type":
		s = "Content-Style-Type"
	case "Content-Transfer-Encoding":
		s = "Content-Transfer-Encoding"
	case "Content-Translation-Type":
		s = "Content-Translation-Type"
	case "Content-Type":
		s = "Content-Type"
	case "Content-Version":
		s = "Content-Version"
	case "Control":
		s = "Control"
	case "Conversion":
		s = "Conversion"
	case "Conversion-With-Loss":
		s = "Conversion-With-Loss"
	case "Cookie":
		s = "Cookie"
	case "Cookie2":
		s = "Cookie2"
	case "Cross-Origin-Embedder-Policy":
		s = "Cross-Origin-Embedder-Policy"
	case "Cross-Origin-Embedder-Policy-Report-Only":
		s = "Cross-Origin-Embedder-Policy-Report-Only"
	case "Cross-Origin-Opener-Policy":
		s = "Cross-Origin-Opener-Policy"
	case "Cross-Origin-Opener-Policy-Report-Only":
		s = "Cross-Origin-Opener-Policy-Report-Only"
	case "Cross-Origin-Resource-Policy":
		s = "Cross-Origin-Resource-Policy"
	case "Dasl":
		s = "Dasl"
	case "Date":
		s = "Date"
	case "Date-Received":
		s = "Date-Received"
	case "Dav":
		s = "Dav"
	case "Default-Style":
		s = "Default-Style"
	case "Deferred-Delivery":
		s = "Deferred-Delivery"
	case "Delivery-Date":
		s = "Delivery-Date"
	case "Delta-Base":
		s = "Delta-Base"
	case "Depth":
		s = "Depth"
	case "Derived-From":
		s = "Derived-From"
	case "Destination":
		s = "Destination"
	case "Differential-Id":
		s = "Differential-Id"
	case "Digest":
		s = "Digest"
	case "Discarded-X400-Ipms-Extensions":
		s = "Discarded-X400-Ipms-Extensions"
	case "Discarded-X400-Mts-Extensions":
		s = "Discarded-X400-Mts-Extensions"
	case "Disclose-Recipients":
		s = "Disclose-Recipients"
	case "Disposition-Notification-Options":
		s = "Disposition-Notification-Options"
	case "Disposition-Notification-To":
		s = "Disposition-Notification-To"
	case "Distribution":
		s = "Distribution"
	case "Dkim-Signature":
		s = "Dkim-Signature"
	case "Dl-Expansion-History":
		s = "Dl-Expansion-History"
	case "Dnt":
		s = "Dnt"
	case "Downgraded-Bcc":
		s = "Downgraded-Bcc"
	case "Downgraded-Cc":
		s = "Downgraded-Cc"
	case "Downgraded-Disposition-Notification-To":
		s = "Downgraded-Disposition-Notification-To"
	case "Downgraded-Final-Recipient":
		s = "Downgraded-Final-Recipient"
	case "Downgraded-From":
		s = "Downgraded-From"
	case "Downgraded-In-Reply-To":
		s = "Downgraded-In-Reply-To"
	case "Downgraded-Mail-From":
		s = "Downgraded-Mail-From"
	case "Downgraded-Message-Id":
		s = "Downgraded-Message-Id"
	case "Downgraded-Original-Recipient":
		s = "Downgraded-Original-Recipient"
	case "Downgraded-Rcpt-To":
		s = "Downgraded-Rcpt-To"
	case "Downgraded-References":
		s = "Downgraded-References"
	case "Downgraded-Reply-To":
		s = "Downgraded-Reply-To"
	case "Downgraded-Resent-Bcc":
		s = "Downgraded-Resent-Bcc"
	case "Downgraded-Resent-Cc":
		s = "Downgraded-Resent-Cc"
	case "Downgraded-Resent-From":
		s = "Downgraded-Resent-From"
	case "Downgraded-Resent-Reply-To":
		s = "Downgraded-Resent-Reply-To"
	case "Downgraded-Resent-Sender":
		s = "Downgraded-Resent-Sender"
	case "Downgraded-Resent-To":
		s = "Downgraded-Resent-To"
	case "Downgraded-Return-Path":
		s = "Downgraded-Return-Path"
	case "Downgraded-Sender":
		s = "Downgraded-Sender"
	case "Downgraded-To":
		s = "Downgraded-To"
	case "Early-Data":
		s = "Early-Data"
	case "Ediint-Features":
		s = "Ediint-Features"
	case "Encoding":
		s = "Encoding"
	case "Encrypted":
		s = "Encrypted"
	case "Etag":
		s = "Etag"
	case "Expect":
		s = "Expect"
	case "Expect-Ct":
		s = "Expect-Ct"
	case "Expires":
		s = "Expires"
	case "Expiry-Date":
		s = "Expiry-Date"
	case "Ext":
		s = "Ext"
	case "Followup-To":
		s = "Followup-To"
	case "Forwarded":
		s = "Forwarded"
	case "From":
		s = "From"
	case "Generate-Delivery-Report":
		s = "Generate-Delivery-Report"
	case "Getprofile":
		s = "Getprofile"
	case "Hobareg":
		s = "Hobareg"
	case "Host":
		s = "Host"
	case "Http2-Settings":
		s = "Http2-Settings"
	case "If":
		s = "If"
	case "If-Match":
		s = "If-Match"
	case "If-Modified-Since":
		s = "If-Modified-Since"
	case "If-None-Match":
		s = "If-None-Match"
	case "If-Range":
		s = "If-Range"
	case "If-Schedule-Tag-Match":
		s = "If-Schedule-Tag-Match"
	case "If-Unmodified-Since":
		s = "If-Unmodified-Since"
	case "Im":
		s = "Im"
	case "Importance":
		s = "Importance"
	case "In-Reply-To":
		s = "In-Reply-To"
	case "Include-Referred-Token-Binding-Id":
		s = "Include-Referred-Token-Binding-Id"
	case "Incomplete-Copy":
		s = "Incomplete-Copy"
	case "Injection-Date":
		s = "Injection-Date"
	case "Injection-Info":
		s = "Injection-Info"
	case "Isolation":
		s = "Isolation"
	case "Keep-Alive":
		s = "Keep-Alive"
	case "Keywords":
		s = "Keywords"
	case "Label":
		s = "Label"
	case "Language":
		s = "Language"
	case "Last-Event-Id":
		s = "Last-Event-Id"
	case "Last-Modified":
		s = "Last-Modified"
	case "Latest-Delivery-Time":
		s = "Latest-Delivery-Time"
	case "Lines":
		s = "Lines"
	case "Link":
		s = "Link"
	case "List-Archive":
		s = "List-Archive"
	case "List-Help":
		s = "List-Help"
	case "List-Id":
		s = "List-Id"
	case "List-Owner":
		s = "List-Owner"
	case "List-Post":
		s = "List-Post"
	case "List-Subscribe":
		s = "List-Subscribe"
	case "List-Unsubscribe":
		s = "List-Unsubscribe"
	case "List-Unsubscribe-Post":
		s = "List-Unsubscribe-Post"
	case "Location":
		s = "Location"
	case "Lock-Token":
		s = "Lock-Token"
	case "Man":
		s = "Man"
	case "Max-Forwards":
		s = "Max-Forwards"
	case "Memento-Datetime":
		s = "Memento-Datetime"
	case "Message-Context":
		s = "Message-Context"
	case "Message-Id":
		s = "Message-Id"
	case "Message-Type":
		s = "Message-Type"
	case "Meter":
		s = "Meter"
	case "Method-Check":
		s = "Method-Check"
	case "Method-Check-Expires":
		s = "Method-Check-Expires"
	case "Mime-Version":
		s = "Mime-Version"
	case "Mmhs-Acp127-Message-Identifier":
		s = "Mmhs-Acp127-Message-Identifier"
	case "Mmhs-Codress-Message-Indicator":
		s = "Mmhs-Codress-Message-Indicator"
	case "Mmhs-Copy-Precedence":
		s = "Mmhs-Copy-Precedence"
	case "Mmhs-Exempted-Address":
		s = "Mmhs-Exempted-Address"
	case "Mmhs-Extended-Authorisation-Info":
		s = "Mmhs-Extended-Authorisation-Info"
	case "Mmhs-Handling-Instructions":
		s = "Mmhs-Handling-Instructions"
	case "Mmhs-Message-Instructions":
		s = "Mmhs-Message-Instructions"
	case "Mmhs-Message-Type":
		s = "Mmhs-Message-Type"
	case "Mmhs-Originator-Plad":
		s = "Mmhs-Originator-Plad"
	case "Mmhs-Originator-Reference":
		s = "Mmhs-Originator-Reference"
	case "Mmhs-Other-Recipients-Indicator-Cc":
		s = "Mmhs-Other-Recipients-Indicator-Cc"
	case "Mmhs-Other-Recipients-Indicator-To":
		s = "Mmhs-Other-Recipients-Indicator-To"
	case "Mmhs-Primary-Precedence":
		s = "Mmhs-Primary-Precedence"
	case "Mmhs-Subject-Indicator-Codes":
		s = "Mmhs-Subject-Indicator-Codes"
	case "Mt-Priority":
		s = "Mt-Priority"
	case "Negotiate":
		s = "Negotiate"
	case "Newsgroups":
		s = "Newsgroups"
	case "Nntp-Posting-Date":
		s = "Nntp-Posting-Date"
	case "Nntp-Posting-Host":
		s = "Nntp-Posting-Host"
	case "Obsoletes":
		s = "Obsoletes"
	case "Odata-Entityid":
		s = "Odata-Entityid"
	case "Odata-Isolation":
		s = "Odata-Isolation"
	case "Odata-Maxversion":
		s = "Odata-Maxversion"
	case "Odata-Version":
		s = "Odata-Version"
	case "Opt":
		s = "Opt"
	case "Optional-Www-Authenticate":
		s = "Optional-Www-Authenticate"
	case "Ordering-Type":
		s = "Ordering-Type"
	case "Organization":
		s = "Organization"
	case "Origin":
		s = "Origin"
	case "Origin-Agent-Cluster":
		s = "Origin-Agent-Cluster"
	case "Original-Encoded-Information-Types":
		s = "Original-Encoded-Information-Types"
	case "Original-From":
		s = "Original-From"
	case "Original-Message-Id":
		s = "Original-Message-Id"
	case "Original-Recipient":
		s = "Original-Recipient"
	case "Original-Sender":
		s = "Original-Sender"
	case "Original-Subject":
		s = "Original-Subject"
	case "Originator-Return-Address":
		s = "Originator-Return-Address"
	case "Oscore":
		s = "Oscore"
	case "Oslc-Core-Version":
		s = "Oslc-Core-Version"
	case "Overwrite":
		s = "Overwrite"
	case "P3p":
		s = "P3p"
	case "Path":
		s = "Path"
	case "Pep":
		s = "Pep"
	case "Pep-Info":
		s = "Pep-Info"
	case "Pics-Label":
		s = "Pics-Label"
	case "Ping-From":
		s = "Ping-From"
	case "Ping-To":
		s = "Ping-To"
	case "Position":
		s = "Position"
	case "Posting-Version":
		s = "Posting-Version"
	case "Pragma":
		s = "Pragma"
	case "Prefer":
		s = "Prefer"
	case "Preference-Applied":
		s = "Preference-Applied"
	case "Prevent-Nondelivery-Report":
		s = "Prevent-Nondelivery-Report"
	case "Priority":
		s = "Priority"
	case "Profileobject":
		s = "Profileobject"
	case "Protocol":
		s = "Protocol"
	case "Protocol-Info":
		s = "Protocol-Info"
	case "Protocol-Query":
		s = "Protocol-Query"
	case "Protocol-Request":
		s = "Protocol-Request"
	case "Proxy-Authenticate":
		s = "Proxy-Authenticate"
	case "Proxy-Authentication-Info":
		s = "Proxy-Authentication-Info"
	case "Proxy-Authorization":
		s = "Proxy-Authorization"
	case "Proxy-Features":
		s = "Proxy-Features"
	case "Proxy-Instruction":
		s = "Proxy-Instruction"
	case "Proxy-Status":
		s = "Proxy-Status"
	case "Public":
		s = "Public"
	case "Public-Key-Pins":
		s = "Public-Key-Pins"
	case "Public-Key-Pins-Report-Only":
		s = "Public-Key-Pins-Report-Only"
	case "Range":
		s = "Range"
	case "Received":
		s = "Received"
	case "Received-Spf":
		s = "Received-Spf"
	case "Redirect-Ref":
		s = "Redirect-Ref"
	case "References":
		s = "References"
	case "Referer":
		s = "Referer"
	case "Referer-Root":
		s = "Referer-Root"
	case "Refresh":
		s = "Refresh"
	case "Relay-Version":
		s = "Relay-Version"
	case "Repeatability-Client-Id":
		s = "Repeatability-Client-Id"
	case "Repeatability-First-Sent":
		s = "Repeatability-First-Sent"
	case "Repeatability-Request-Id":
		s = "Repeatability-Request-Id"
	case "Repeatability-Result":
		s = "Repeatability-Result"
	case "Replay-Nonce":
		s = "Replay-Nonce"
	case "Reply-By":
		s = "Reply-By"
	case "Reply-To":
		s = "Reply-To"
	case "Require-Recipient-Valid-Since":
		s = "Require-Recipient-Valid-Since"
	case "Resent-Bcc":
		s = "Resent-Bcc"
	case "Resent-Cc":
		s = "Resent-Cc"
	case "Resent-Date":
		s = "Resent-Date"
	case "Resent-From":
		s = "Resent-From"
	case "Resent-Message-Id":
		s = "Resent-Message-Id"
	case "Resent-Reply-To":
		s = "Resent-Reply-To"
	case "Resent-Sender":
		s = "Resent-Sender"
	case "Resent-To":
		s = "Resent-To"
	case "Retry-After":
		s = "Retry-After"
	case "Return-Path":
		s = "Return-Path"
	case "Safe":
		s = "Safe"
	case "Schedule-Reply":
		s = "Schedule-Reply"
	case "Schedule-Tag":
		s = "Schedule-Tag"
	case "Sec-Gpc":
		s = "Sec-Gpc"
	case "Sec-Purpose":
		s = "Sec-Purpose"
	case "Sec-Token-Binding":
		s = "Sec-Token-Binding"
	case "Sec-Websocket-Accept":
		s = "Sec-Websocket-Accept"
	case "Sec-Websocket-Extensions":
		s = "Sec-Websocket-Extensions"
	case "Sec-Websocket-Key":
		s = "Sec-Websocket-Key"
	case "Sec-Websocket-Protocol":
		s = "Sec-Websocket-Protocol"
	case "Sec-Websocket-Version":
		s = "Sec-Websocket-Version"
	case "Security-Scheme":
		s = "Security-Scheme"
	case "See-Also":
		s = "See-Also"
	case "Sender":
		s = "Sender"
	case "Sensitivity":
		s = "Sensitivity"
	case "Server":
		s = "Server"
	case "Server-Timing":
		s = "Server-Timing"
	case "Set-Cookie":
		s = "Set-Cookie"
	case "Set-Cookie2":
		s = "Set-Cookie2"
	case "Setprofile":
		s = "Setprofile"
	case "Slug":
		s = "Slug"
	case "Soapaction":
		s = "Soapaction"
	case "Solicitation":
		s = "Solicitation"
	case "Status-Uri":
		s = "Status-Uri"
	case "Strict-Transport-Security":
		s = "Strict-Transport-Security"
	case "Subect":
		s = "Subect"
	case "Subject":
		s = "Subject"
	case "Summary":
		s = "Summary"
	case "Sunset":
		s = "Sunset"
	case "Supersedes":
		s = "Supersedes"
	case "Surrogate-Capability":
		s = "Surrogate-Capability"
	case "Surrogate-Control":
		s = "Surrogate-Control"
	case "Tcn":
		s = "Tcn"
	case "Te":
		s = "Te"
	case "Timeout":
		s = "Timeout"
	case "Timing-Allow-Origin":
		s = "Timing-Allow-Origin"
	case "Tk":
		s = "Tk"
	case "Tls-Report-Domain":
		s = "Tls-Report-Domain"
	case "Tls-Report-Submitter":
		s = "Tls-Report-Submitter"
	case "Tls-Required":
		s = "Tls-Required"
	case "To":
		s = "To"
	case "Topic":
		s = "Topic"
	case "Traceparent":
		s = "Traceparent"
	case "Tracestate":
		s = "Tracestate"
	case "Trailer":
		s = "Trailer"
	case "Transfer-Encoding":
		s = "Transfer-Encoding"
	case "Ttl":
		s = "Ttl"
	case "Upgrade":
		s = "Upgrade"
	case "Upgrade-Insecure-Requests":
		s = "Upgrade-Insecure-Requests"
	case "Urgency":
		s = "Urgency"
	case "Uri":
		s = "Uri"
	case "User-Agent":
		s = "User-Agent"
	case "Variant-Vary":
		s = "Variant-Vary"
	case "Vary":
		s = "Vary"
	case "Vbr-Info":
		s = "Vbr-Info"
	case "Via":
		s = "Via"
	case "Want-Digest":
		s = "Want-Digest"
	case "Warning":
		s = "Warning"
	case "Www-Authenticate":
		s = "Www-Authenticate"
	case "X-B3-Spanid":
		s = "X-B3-Spanid"
	case "X-B3-Traceid":
		s = "X-B3-Traceid"
	case "X-Content-Type-Options":
		s = "X-Content-Type-Options"
	case "X-Correlation-Id":
		s = "X-Correlation-Id"
	case "X-Forwarded-For":
		s = "X-Forwarded-For"
	case "X-Forwarded-Host":
		s = "X-Forwarded-Host"
	case "X-Forwarded-Proto":
		s = "X-Forwarded-Proto"
	case "X-Frame-Options":
		s = "X-Frame-Options"
	case "X-Imforwards":
		s = "X-Imforwards"
	case "X-Powered-By":
		s = "X-Powered-By"
	case "X-Real-Ip":
		s = "X-Real-Ip"
	case "X-Request-Id":
		s = "X-Request-Id"
	case "X-Requested-With":
		s = "X-Requested-With"
	case "X400-Content-Identifier":
		s = "X400-Content-Identifier"
	case "X400-Content-Return":
		s = "X400-Content-Return"
	case "X400-Content-Type":
		s = "X400-Content-Type"
	case "X400-Mts-Identifier":
		s = "X400-Mts-Identifier"
	case "X400-Originator":
		s = "X400-Originator"
	case "X400-Received":
		s = "X400-Received"
	case "X400-Recipients":
		s = "X400-Recipients"
	case "X400-Trace":
		s = "X400-Trace"
	case "Xref":
		s = "Xref"
	}
	return
}
