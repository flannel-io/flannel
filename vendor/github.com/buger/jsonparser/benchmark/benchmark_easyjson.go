package benchmark

import (
	json "encoding/json"
	jlexer "github.com/mailru/easyjson/jlexer"
	jwriter "github.com/mailru/easyjson/jwriter"
)

var _ = json.RawMessage{} // suppress unused package warning

func easyjson_decode_github_com_buger_jsonparser_benchmark_LargePayload(in *jlexer.Lexer, out *LargePayload) {
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "users":
			in.Delim('[')
			if !in.IsDelim(']') {
				out.Users = make([]*DSUser, 0, 8)
			} else {
				out.Users = nil
			}
			for !in.IsDelim(']') {
				var v1 *DSUser
				if in.IsNull() {
					in.Skip()
					v1 = nil
				} else {
					v1 = new(DSUser)
					(*v1).UnmarshalEasyJSON(in)
				}
				out.Users = append(out.Users, v1)
				in.WantComma()
			}
			in.Delim(']')
		case "topics":
			if in.IsNull() {
				in.Skip()
				out.Topics = nil
			} else {
				out.Topics = new(DSTopicsList)
				(*out.Topics).UnmarshalEasyJSON(in)
			}
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
}
func easyjson_encode_github_com_buger_jsonparser_benchmark_LargePayload(out *jwriter.Writer, in *LargePayload) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"users\":")
	out.RawByte('[')
	for v2, v3 := range in.Users {
		if v2 > 0 {
			out.RawByte(',')
		}
		if v3 == nil {
			out.RawString("null")
		} else {
			(*v3).MarshalEasyJSON(out)
		}
	}
	out.RawByte(']')
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"topics\":")
	if in.Topics == nil {
		out.RawString("null")
	} else {
		(*in.Topics).MarshalEasyJSON(out)
	}
	out.RawByte('}')
}
func (v *LargePayload) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson_encode_github_com_buger_jsonparser_benchmark_LargePayload(w, v)
}
func (v *LargePayload) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson_decode_github_com_buger_jsonparser_benchmark_LargePayload(l, v)
}
func easyjson_decode_github_com_buger_jsonparser_benchmark_DSTopicsList(in *jlexer.Lexer, out *DSTopicsList) {
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "topics":
			in.Delim('[')
			if !in.IsDelim(']') {
				out.Topics = make([]*DSTopic, 0, 8)
			} else {
				out.Topics = nil
			}
			for !in.IsDelim(']') {
				var v4 *DSTopic
				if in.IsNull() {
					in.Skip()
					v4 = nil
				} else {
					v4 = new(DSTopic)
					(*v4).UnmarshalEasyJSON(in)
				}
				out.Topics = append(out.Topics, v4)
				in.WantComma()
			}
			in.Delim(']')
		case "more_topics_url":
			out.MoreTopicsUrl = in.String()
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
}
func easyjson_encode_github_com_buger_jsonparser_benchmark_DSTopicsList(out *jwriter.Writer, in *DSTopicsList) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"topics\":")
	out.RawByte('[')
	for v5, v6 := range in.Topics {
		if v5 > 0 {
			out.RawByte(',')
		}
		if v6 == nil {
			out.RawString("null")
		} else {
			(*v6).MarshalEasyJSON(out)
		}
	}
	out.RawByte(']')
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"more_topics_url\":")
	out.String(in.MoreTopicsUrl)
	out.RawByte('}')
}
func (v *DSTopicsList) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson_encode_github_com_buger_jsonparser_benchmark_DSTopicsList(w, v)
}
func (v *DSTopicsList) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson_decode_github_com_buger_jsonparser_benchmark_DSTopicsList(l, v)
}
func easyjson_decode_github_com_buger_jsonparser_benchmark_DSTopic(in *jlexer.Lexer, out *DSTopic) {
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "id":
			out.Id = in.Int()
		case "slug":
			out.Slug = in.String()
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
}
func easyjson_encode_github_com_buger_jsonparser_benchmark_DSTopic(out *jwriter.Writer, in *DSTopic) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"id\":")
	out.Int(in.Id)
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"slug\":")
	out.String(in.Slug)
	out.RawByte('}')
}
func (v *DSTopic) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson_encode_github_com_buger_jsonparser_benchmark_DSTopic(w, v)
}
func (v *DSTopic) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson_decode_github_com_buger_jsonparser_benchmark_DSTopic(l, v)
}
func easyjson_decode_github_com_buger_jsonparser_benchmark_DSUser(in *jlexer.Lexer, out *DSUser) {
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "username":
			out.Username = in.String()
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
}
func easyjson_encode_github_com_buger_jsonparser_benchmark_DSUser(out *jwriter.Writer, in *DSUser) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"username\":")
	out.String(in.Username)
	out.RawByte('}')
}
func (v *DSUser) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson_encode_github_com_buger_jsonparser_benchmark_DSUser(w, v)
}
func (v *DSUser) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson_decode_github_com_buger_jsonparser_benchmark_DSUser(l, v)
}
func easyjson_decode_github_com_buger_jsonparser_benchmark_MediumPayload(in *jlexer.Lexer, out *MediumPayload) {
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "person":
			if in.IsNull() {
				in.Skip()
				out.Person = nil
			} else {
				out.Person = new(CBPerson)
				(*out.Person).UnmarshalEasyJSON(in)
			}
		case "company":
			in.Delim('{')
			if !in.IsDelim('}') {
				out.Company = make(map[string]interface{})
			} else {
				out.Company = nil
			}
			for !in.IsDelim('}') {
				key := in.String()
				in.WantColon()
				var v7 interface{}
				v7 = in.Interface()
				(out.Company)[key] = v7
				in.WantComma()
			}
			in.Delim('}')
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
}
func easyjson_encode_github_com_buger_jsonparser_benchmark_MediumPayload(out *jwriter.Writer, in *MediumPayload) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"person\":")
	if in.Person == nil {
		out.RawString("null")
	} else {
		(*in.Person).MarshalEasyJSON(out)
	}
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"company\":")
	out.RawByte('{')
	v8_first := true
	for v8_name, v8_value := range in.Company {
		if !v8_first {
			out.RawByte(',')
		}
		v8_first = false
		out.String(v8_name)
		out.Raw(json.Marshal(v8_value))
	}
	out.RawByte('}')
	out.RawByte('}')
}
func (v *MediumPayload) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson_encode_github_com_buger_jsonparser_benchmark_MediumPayload(w, v)
}
func (v *MediumPayload) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson_decode_github_com_buger_jsonparser_benchmark_MediumPayload(l, v)
}
func easyjson_decode_github_com_buger_jsonparser_benchmark_CBPerson(in *jlexer.Lexer, out *CBPerson) {
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "name":
			if in.IsNull() {
				in.Skip()
				out.Name = nil
			} else {
				out.Name = new(CBName)
				(*out.Name).UnmarshalEasyJSON(in)
			}
		case "github":
			if in.IsNull() {
				in.Skip()
				out.Github = nil
			} else {
				out.Github = new(CBGithub)
				(*out.Github).UnmarshalEasyJSON(in)
			}
		case "gravatar":
			if in.IsNull() {
				in.Skip()
				out.Gravatar = nil
			} else {
				out.Gravatar = new(CBGravatar)
				(*out.Gravatar).UnmarshalEasyJSON(in)
			}
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
}
func easyjson_encode_github_com_buger_jsonparser_benchmark_CBPerson(out *jwriter.Writer, in *CBPerson) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"name\":")
	if in.Name == nil {
		out.RawString("null")
	} else {
		(*in.Name).MarshalEasyJSON(out)
	}
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"github\":")
	if in.Github == nil {
		out.RawString("null")
	} else {
		(*in.Github).MarshalEasyJSON(out)
	}
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"gravatar\":")
	if in.Gravatar == nil {
		out.RawString("null")
	} else {
		(*in.Gravatar).MarshalEasyJSON(out)
	}
	out.RawByte('}')
}
func (v *CBPerson) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson_encode_github_com_buger_jsonparser_benchmark_CBPerson(w, v)
}
func (v *CBPerson) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson_decode_github_com_buger_jsonparser_benchmark_CBPerson(l, v)
}
func easyjson_decode_github_com_buger_jsonparser_benchmark_CBName(in *jlexer.Lexer, out *CBName) {
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "full_name":
			out.FullName = in.String()
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
}
func easyjson_encode_github_com_buger_jsonparser_benchmark_CBName(out *jwriter.Writer, in *CBName) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"full_name\":")
	out.String(in.FullName)
	out.RawByte('}')
}
func (v *CBName) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson_encode_github_com_buger_jsonparser_benchmark_CBName(w, v)
}
func (v *CBName) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson_decode_github_com_buger_jsonparser_benchmark_CBName(l, v)
}
func easyjson_decode_github_com_buger_jsonparser_benchmark_CBGithub(in *jlexer.Lexer, out *CBGithub) {
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "followers":
			out.Followers = in.Int()
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
}
func easyjson_encode_github_com_buger_jsonparser_benchmark_CBGithub(out *jwriter.Writer, in *CBGithub) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"followers\":")
	out.Int(in.Followers)
	out.RawByte('}')
}
func (v *CBGithub) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson_encode_github_com_buger_jsonparser_benchmark_CBGithub(w, v)
}
func (v *CBGithub) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson_decode_github_com_buger_jsonparser_benchmark_CBGithub(l, v)
}
func easyjson_decode_github_com_buger_jsonparser_benchmark_CBGravatar(in *jlexer.Lexer, out *CBGravatar) {
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "avatars":
			in.Delim('[')
			if !in.IsDelim(']') {
				out.Avatars = make([]*CBAvatar, 0, 8)
			} else {
				out.Avatars = nil
			}
			for !in.IsDelim(']') {
				var v9 *CBAvatar
				if in.IsNull() {
					in.Skip()
					v9 = nil
				} else {
					v9 = new(CBAvatar)
					(*v9).UnmarshalEasyJSON(in)
				}
				out.Avatars = append(out.Avatars, v9)
				in.WantComma()
			}
			in.Delim(']')
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
}
func easyjson_encode_github_com_buger_jsonparser_benchmark_CBGravatar(out *jwriter.Writer, in *CBGravatar) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"avatars\":")
	out.RawByte('[')
	for v10, v11 := range in.Avatars {
		if v10 > 0 {
			out.RawByte(',')
		}
		if v11 == nil {
			out.RawString("null")
		} else {
			(*v11).MarshalEasyJSON(out)
		}
	}
	out.RawByte(']')
	out.RawByte('}')
}
func (v *CBGravatar) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson_encode_github_com_buger_jsonparser_benchmark_CBGravatar(w, v)
}
func (v *CBGravatar) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson_decode_github_com_buger_jsonparser_benchmark_CBGravatar(l, v)
}
func easyjson_decode_github_com_buger_jsonparser_benchmark_CBAvatar(in *jlexer.Lexer, out *CBAvatar) {
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "url":
			out.Url = in.String()
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
}
func easyjson_encode_github_com_buger_jsonparser_benchmark_CBAvatar(out *jwriter.Writer, in *CBAvatar) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"url\":")
	out.String(in.Url)
	out.RawByte('}')
}
func (v *CBAvatar) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson_encode_github_com_buger_jsonparser_benchmark_CBAvatar(w, v)
}
func (v *CBAvatar) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson_decode_github_com_buger_jsonparser_benchmark_CBAvatar(l, v)
}
func easyjson_decode_github_com_buger_jsonparser_benchmark_SmallPayload(in *jlexer.Lexer, out *SmallPayload) {
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "st":
			out.St = in.Int()
		case "sid":
			out.Sid = in.Int()
		case "tt":
			out.Tt = in.String()
		case "gr":
			out.Gr = in.Int()
		case "uuid":
			out.Uuid = in.String()
		case "ip":
			out.Ip = in.String()
		case "ua":
			out.Ua = in.String()
		case "tz":
			out.Tz = in.Int()
		case "v":
			out.V = in.Int()
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
}
func easyjson_encode_github_com_buger_jsonparser_benchmark_SmallPayload(out *jwriter.Writer, in *SmallPayload) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"st\":")
	out.Int(in.St)
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"sid\":")
	out.Int(in.Sid)
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"tt\":")
	out.String(in.Tt)
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"gr\":")
	out.Int(in.Gr)
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"uuid\":")
	out.String(in.Uuid)
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"ip\":")
	out.String(in.Ip)
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"ua\":")
	out.String(in.Ua)
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"tz\":")
	out.Int(in.Tz)
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"v\":")
	out.Int(in.V)
	out.RawByte('}')
}
func (v *SmallPayload) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson_encode_github_com_buger_jsonparser_benchmark_SmallPayload(w, v)
}
func (v *SmallPayload) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson_decode_github_com_buger_jsonparser_benchmark_SmallPayload(l, v)
}
