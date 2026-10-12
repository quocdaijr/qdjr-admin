-- apps/api/migrations/0009_seed_site_content.sql
-- +goose Up
-- Generated once by apps/site/scripts/export-seed.ts from the content that used to ship
-- inside the Nuxt site (content/blog/*.md, app/data/profile.*.ts, SocialLinks.vue).
-- Idempotent: inserts are "on conflict do nothing"; singleton updates only fill empty rows.

-- +goose StatementBegin
update public.profile set
    full_name = 'Nguyễn Quốc Đại',
    tagline = 'Senior Backend Software Engineer',
    bio = 'Backend Software Engineer with 6+ years of experience building and operating high-performance backend systems for media and e-commerce platforms. Strong background in system design, performance optimization, background processing, and third-party integrations. Experienced in owning and operating production systems at scale.',
    email = 'quocdaijr@gmail.com',
    social_links = '{"mail":"mailto:quocdaijr@gmail.com","telegram":"https://t.me/quocdaijr","signal":"https://signal.me/#eu/U1eIvUn29-2TB8fu6lTq-nxX0qOa3LNUCoyywDx_WVvEdYF7C5NK15eKfieS3TDe","discord":"https://discord.com/users/quocdaijr","github":"https://github.com/quocdaijr","facebook":"https://facebook.com/quocdaijr","linkedin":"https://www.linkedin.com/in/quocdaijr"}'::jsonb,
    content = '{"vi":{"profile":{"name":"Nguyễn Quốc Đại","displayName":"Nguyễn Quốc Đại","role":"Senior Backend Software Engineer","photo":"/profile.jpg","photoAlt":"Nguyễn Quốc Đại - Senior Backend Software Engineer","summary":"Backend Software Engineer với hơn 6 năm build và vận hành hệ thống backend high-performance cho các nền tảng báo chí và e-commerce. Mạnh về system design, tối ưu performance, background job và tích hợp third-party. Đã trực tiếp lo và vận hành nhiều hệ thống production quy mô lớn."},"quote":{"text":"Giải quyết vấn đề trước, rồi mới viết code","author":"John Johnson"},"skills":[{"group":"Programming Languages","expert":"PHP (Yii2, Laravel)","proficient":"Node.js (NestJS, AdonisJS), Python, JavaScript"},{"group":"Database & Storage","expert":"MySQL, Redis, Elasticsearch","proficient":"MongoDB, Database optimization, Query performance tuning"},{"group":"Messaging & Streaming","expert":"RabbitMQ","proficient":"Kafka"},{"group":"DevOps & Infrastructure","expert":"Linux, Docker, Nginx, Git, Supervisor","proficient":"Kubernetes, CI/CD Pipelines, AWS"},{"group":"Frontend Technologies","proficient":"Vue.js, Nuxt.js, HTML5, CSS3, Tailwind CSS"},{"group":"Third-party Integrations","note":"Shopify, E-commerce platforms, Payment gateways, Analytics & tracking services, Customer engagement tools, Real-time communication systems, Project management platforms, API testing tools"}],"otherSkills":[{"label":"Vận hành production","text":"Có kinh nghiệm observability, monitoring và xử lý incident trên production"},{"label":"Teamwork","text":"Giao tiếp tốt, chủ động share kiến thức kỹ thuật khi làm việc với nhiều team"},{"label":"Problem solving","text":"Phân tích và debug tốt các hệ thống phức tạp, thiết kế giải pháp dễ scale cho ứng dụng traffic lớn"},{"label":"Tự học","text":"Luôn cập nhật công nghệ mới và áp dụng best practice vào công việc"},{"label":"Sở thích","text":"Bóng đá, chạy bộ, công nghệ và system architecture"}],"timeline":[{"period":"08/2014 - 12/2018","org":"Trường Đại học Tài nguyên và Môi trường TP.HCM","short":"HCMUNRE","kind":"education","major":"Công nghệ Thông tin","degree":"Kỹ sư","achievements":[]},{"period":"08/2018 - 07/2019","org":"Applancer JSC - Onsite tại Báo Tuổi Trẻ","short":"Applancer","kind":"work","position":"Web Developer","technologies":"PHP, Yii2 Framework, MySQL, Elasticsearch, Redis, Nginx, RabbitMQ, Git, Linux, Docker, Supervisor","achievements":["Maintain và cải tiến backend của các hệ thống web đang chạy production","Trực tiếp phát triển feature cho các website traffic lớn","Tham gia develop và deploy trên môi trường Linux, Docker"]},{"period":"07/2019 - 08/2022","org":"Báo Tuổi Trẻ","short":"Tuổi Trẻ","kind":"work","position":"Software Development Engineer","technologies":"PHP, Python, Yii2 Framework, Django Rest Framework, MySQL, Elasticsearch, Redis, Memcached, Nginx, RabbitMQ, Git, Linux, Docker, Supervisor, Sentry","achievements":["Maintain và phát triển backend cho các trang tin tức traffic lớn","Thiết kế database schema, build API và các tool nội bộ cho quy trình biên tập nội dung","Build các package, module backend dùng chung để code dễ maintain và phát triển nhanh hơn","Viết background job và tool tự động để sync, chuyển đổi và tổng hợp dữ liệu","Trực tiếp deploy và vận hành production: config service, monitoring, logging để hệ thống chạy ổn định","Nghiên cứu và triển khai Single Sign-On (SSO) dùng chung cho nhiều sản phẩm"]},{"period":"08/2022 - Nay","org":"FireGroup Technology","short":"FireGroup","kind":"work","position":"Senior Backend Software Engineer","technologies":"PHP, Python, Node.js, Laravel Framework, NestJS, MySQL, Redis, RabbitMQ, Kafka, GitLab, Linux, Docker, Kubernetes, Supervisor, Sentry, Rancher","thirdParties":"Shopify, Jira, Segment, CustomerIO, Crisp Chat","achievements":["Thiết kế và phát triển backend service, hệ thống background job cho các Shopify app, tập trung vào performance và độ ổn định","Thiết kế database schema và các thành phần backend core, tham gia planning và ra quyết định kỹ thuật","Phụ trách backend các Shopify app, đảm bảo hệ thống ổn định, dễ scale và chạy liên tục","Tích hợp third-party và service nội bộ để mở rộng tính năng sản phẩm, tối ưu quy trình vận hành","Phối hợp với team DevOps setup deploy trên Kubernetes, tích hợp monitoring và observability cho các backend service","Dùng AI tool cho việc code, debug và tăng năng suất hằng ngày"]}],"projects":[{"group":"FireGroup Technology","name":"OneMobile ‑ Mobile App Builder","image":"/images/projects/om.webp","alt":"OneMobile","url":"https://onemobile.ai","description":"Biến store thành mobile app với OneMobile: tăng nhận diện thương hiệu, giảm chi phí quảng cáo và giữ chân khách hàng.","role":"Backend Software Engineer - Phát triển backend service, analytics & reporting và tích hợp third-party."},{"group":"FireGroup Technology","name":"OneLoyalty: Loyalty & Rewards","image":"/images/projects/ol.webp","alt":"OneLoyalty","url":"https://oneloyalty.io","description":"Triển khai nhanh chương trình khách hàng thân thiết và referral, giúp tăng doanh số, tỷ lệ quay lại và customer lifetime value.","role":"Backend Software Engineer - Build hệ thống backend và business logic từ đầu."},{"group":"FireGroup Technology","name":"Transcy: AI Language Translate","image":"/images/projects/tc.webp","alt":"Transcy","url":"https://transcy.io","description":"Dịch store bằng OpenAI, DeepL, Gemini, Baidu… Đổi tiền tệ để bán hàng toàn cầu.","role":"Backend Software Engineer - Phát triển translation service, tối ưu performance và tích hợp các AI provider."},{"group":"FireGroup Technology","name":"Swift SEO Page Speed Optimizer","image":"/images/projects/sw.webp","alt":"Swift","url":"https://onecommerce.io/swift","description":"Cải thiện SEO và page speed dễ dàng. Website rank cao, load nhanh thì conversion tốt hơn.","role":"Backend Software Engineer - Rebuild backend và phát triển các tính năng SEO."},{"group":"FireGroup Technology","name":"FireGroup - OneExpert Internal Tools & Systems","image":"/images/projects/fg.webp","alt":"FireGroup Internal","description":"Nền tảng backend nội bộ giúp các team CS, TS và Expert cung cấp dịch vụ Expert trên các app của FireGroup.","role":"Backend Software Engineer - Thiết kế và build các hệ thống backend nội bộ, tích hợp với các sản phẩm của FireGroup."},{"group":"Báo Tuổi Trẻ","name":"SSO Tuoitre Authentication System","image":"/images/projects/tt.webp","alt":"SSO Tuoitre","url":"https://sso.tuoitre.vn","description":"Nền tảng Single Sign-On (SSO) cho các dịch vụ số của Tuổi Trẻ.","role":"Software Development Engineer - Thiết kế và triển khai kiến trúc SSO, authentication flow và các cơ chế bảo mật."},{"group":"Báo Tuổi Trẻ","name":"Tuoi Tre Rao Vat","image":"/images/projects/ttrv.webp","alt":"Tuoi Tre Rao Vat","url":"https://raovat.tuoitre.vn","description":"Nền tảng rao vặt online của Báo Tuổi Trẻ.","role":"Software Development Engineer - Maintain feature backend hiện có và phát triển feature mới."},{"group":"Báo Tuổi Trẻ","name":"Tuoi Tre Cuoi","image":"/images/projects/ttc.webp","alt":"Tuoi Tre Cuoi","url":"https://cuoi.tuoitre.vn","description":"Chuyên trang nội dung hài, châm biếm của Báo Tuổi Trẻ.","role":"Software Development Engineer - Phát triển và maintain backend quản lý nội dung và bình luận."},{"group":"Báo Tuổi Trẻ","name":"Tuoi Tre News","image":"/images/projects/ttn.webp","alt":"Tuoi Tre News","url":"https://news.tuoitre.vn","description":"Trang tin tiếng Anh của Báo Tuổi Trẻ.","role":"Software Development Engineer - Maintain và cải tiến backend phục vụ phân phối nội dung."},{"group":"Báo Tuổi Trẻ","name":"Tuoi Tre Internal Tools","image":"/images/projects/tt.webp","alt":"Tuoi Tre Internal","description":"Các hệ thống nội bộ phục vụ tòa soạn và vận hành.","role":"Software Development Engineer - Build tool nội bộ từ đầu và migrate các hệ thống cũ sang kiến trúc backend mới."}],"contact":[{"label":"Họ tên","value":"Nguyễn Quốc Đại"},{"label":"Điện thoại","value":"+84969-113-505","href":"tel:+84969113505"},{"label":"Email","value":"quocdaijr@gmail.com","href":"mailto:quocdaijr@gmail.com"},{"label":"Kinh nghiệm","value":"Hơn 6 năm làm web"},{"label":"Vị trí","value":"Senior Backend Software Engineer"},{"label":"Ngày sinh","value":"27/10/1996"},{"label":"Địa chỉ","value":"Xã Mỹ Chánh, Huyện Phù Mỹ, Tỉnh Bình Định"}]},"en":{"profile":{"name":"Nguyen Quoc Dai","displayName":"Quoc Dai Nguyen","role":"Senior Backend Software Engineer","photo":"/profile.jpg","photoAlt":"Quoc Dai Nguyen - Senior Backend Software Engineer Profile Photo","summary":"Backend Software Engineer with 6+ years of experience building and operating high-performance backend systems for media and e-commerce platforms. Strong background in system design, performance optimization, background processing, and third-party integrations. Experienced in owning and operating production systems at scale."},"quote":{"text":"First, solve the problem. Then, write the code","author":"John Johnson"},"skills":[{"group":"Programming Languages","expert":"PHP (Yii2, Laravel)","proficient":"Node.js (NestJS, AdonisJS), Python, JavaScript"},{"group":"Database & Storage","expert":"MySQL, Redis, Elasticsearch","proficient":"MongoDB, Database optimization, Query performance tuning"},{"group":"Messaging & Streaming","expert":"RabbitMQ","proficient":"Kafka"},{"group":"DevOps & Infrastructure","expert":"Linux, Docker, Nginx, Git, Supervisor","proficient":"Kubernetes, CI/CD Pipelines, AWS"},{"group":"Frontend Technologies","proficient":"Vue.js, Nuxt.js, HTML5, CSS3, Tailwind CSS"},{"group":"Third-party Integrations","note":"Shopify, E-commerce platforms, Payment gateways, Analytics & tracking services, Customer engagement tools, Real-time communication systems, Project management platforms, API testing tools"}],"otherSkills":[{"label":"Production Readiness","text":"Experienced in system observability, monitoring, and incident response for production environments"},{"label":"Team Collaboration","text":"Strong communication skills and technical knowledge sharing across cross-functional teams"},{"label":"Problem Solving","text":"Strong analytical skills in debugging complex systems and architecting scalable solutions for high-traffic applications"},{"label":"Continuous Learning","text":"Actively stay current with emerging technologies and implement modern development practices"},{"label":"Interests","text":"Sports (Football, Running), Technology trends, System architecture"}],"timeline":[{"period":"08/2014 - 12/2018","org":"HCMC University of Natural Resources and Environment","short":"HCMUNRE","kind":"education","major":"Information Technology","degree":"Engineer''s Degree","achievements":[]},{"period":"08/2018 - 07/2019","org":"Applancer JSC - Onsite at Tuoi Tre Newspaper","short":"Applancer","kind":"work","position":"Web Developer","technologies":"PHP, Yii2 Framework, MySQL, Elasticsearch, Redis, Nginx, RabbitMQ, Git, Linux, Docker, Supervisor","achievements":["Actively maintained and enhanced backend components of production web platforms","Participated directly in developing features for high-traffic websites operating in production environments","Involved in development and deployment workflows within Linux-based and containerized setups"]},{"period":"07/2019 - 08/2022","org":"Tuoi Tre Newspaper","short":"Tuoi Tre","kind":"work","position":"Software Development Engineer","technologies":"PHP, Python, Yii2 Framework, Django Rest Framework, MySQL, Elasticsearch, Redis, Memcached, Nginx, RabbitMQ, Git, Linux, Docker, Supervisor, Sentry","achievements":["Maintained and evolved backend systems for high-traffic news platforms operating in production environments","Designed database schemas and developed backend APIs and internal tools for content and editorial workflows","Built reusable internal packages and modular backend components to improve maintainability and development efficiency","Developed background processing and automation tools for data synchronization, transformation, and aggregation","Implemented and operated production deployments, service configurations, monitoring, and logging to ensure system reliability","Researched and implemented a Single Sign-On (SSO) authentication solution across multiple platforms"]},{"period":"08/2022 - Present","org":"FireGroup Technology","short":"FireGroup","kind":"work","position":"Senior Backend Software Engineer","technologies":"PHP, Python, Node.js, Laravel Framework, NestJS, MySQL, Redis, RabbitMQ, Kafka, GitLab, Linux, Docker, Kubernetes, Supervisor, Sentry, Rancher","thirdParties":"Shopify, Jira, Segment, CustomerIO, Crisp Chat","achievements":["Designed and developed backend services and background processing systems for Shopify-based applications, with a focus on performance and reliability","Architected database schemas and core backend components, contributing to technical planning and implementation decisions across backend systems","Owned and maintained Shopify application backends, ensuring system stability, scalability, and continuous service delivery","Integrated third-party platforms and internal services to extend product capabilities and streamline business workflows","Collaborated with DevOps teams to configure Kubernetes deployments and integrate monitoring and observability for backend services","Utilized AI-assisted development tools to support code generation, debugging, and daily development productivity"]}],"projects":[{"group":"FireGroup Technology","name":"OneMobile ‑ Mobile App Builder","image":"/images/projects/om.webp","alt":"OneMobile","url":"https://onemobile.ai","description":"Turn your store into a mobile app with OneMobile. Scale brand, reduce ad costs & retain customers.","role":"Backend Software Engineer - Backend services, analytics & reporting, third-party integrations."},{"group":"FireGroup Technology","name":"OneLoyalty: Loyalty & Rewards","image":"/images/projects/ol.webp","alt":"OneLoyalty","url":"https://oneloyalty.io","description":"Easily run loyalty and referral programs that drive sales, customer retention rate & lifetime value.","role":"Backend Software Engineer - Built core backend systems and business logic from scratch."},{"group":"FireGroup Technology","name":"Transcy: AI Language Translate","image":"/images/projects/tc.webp","alt":"Transcy","url":"https://transcy.io","description":"Translate store''s language with OpenAI, DeepL, Gemini, Baidu, etc. Convert Currency to sell globally","role":"Backend Software Engineer - Translation services, performance optimization, AI provider integrations."},{"group":"FireGroup Technology","name":"Swift SEO Page Speed Optimizer","image":"/images/projects/sw.webp","alt":"Swift","url":"https://onecommerce.io/swift","description":"Easily boost your SEO and page speed. Websites that rank higher and load faster convert better.","role":"Backend Software Engineer - Backend rebuild and SEO feature development."},{"group":"FireGroup Technology","name":"FireGroup - OneExpert Internal Tools & Systems","image":"/images/projects/fg.webp","alt":"FireGroup Internal","description":"Internal backend platform enabling CS, TS, and Expert teams to deliver Expert services across FireGroup applications.","role":"Backend Software Engineer - Designed and built internal backend systems, integrated across FireGroup products."},{"group":"Tuoi Tre Newspaper","name":"SSO Tuoitre Authentication System","image":"/images/projects/tt.webp","alt":"SSO Tuoitre","url":"https://sso.tuoitre.vn","description":"Centralized single sign-on platform for Tuoi Tre digital services.","role":"Software Development Engineer - Designed and implemented SSO architecture, authentication flows, and security mechanisms."},{"group":"Tuoi Tre Newspaper","name":"Tuoi Tre Rao Vat","image":"/images/projects/ttrv.webp","alt":"Tuoi Tre Rao Vat","url":"https://raovat.tuoitre.vn","description":"Online marketplace platform operated by Tuoi Tre Newspaper.","role":"Software Development Engineer - Maintained existing backend features and developed new functionalities."},{"group":"Tuoi Tre Newspaper","name":"Tuoi Tre Cuoi","image":"/images/projects/ttc.webp","alt":"Tuoi Tre Cuoi","url":"https://cuoi.tuoitre.vn","description":"Humor and satire content platform under Tuoi Tre Newspaper.","role":"Software Development Engineer - Developed and maintained backend features for content and comment management."},{"group":"Tuoi Tre Newspaper","name":"Tuoi Tre News","image":"/images/projects/ttn.webp","alt":"Tuoi Tre News","url":"https://news.tuoitre.vn","description":"English news website of Tuoi Tre Newspaper","role":"Software Development Engineer - Maintained and enhanced backend features to support content delivery."},{"group":"Tuoi Tre Newspaper","name":"Tuoi Tre Internal Tools","image":"/images/projects/tt.webp","alt":"Tuoi Tre Internal","description":"Internal systems supporting newsroom and operational workflows.","role":"Software Development Engineer - Built internal tools from scratch and migrated legacy systems to modern backend architectures."}],"contact":[{"label":"Name","value":"Quoc Dai Nguyen"},{"label":"Phone","value":"+84969-113-505","href":"tel:+84969113505"},{"label":"Email","value":"quocdaijr@gmail.com","href":"mailto:quocdaijr@gmail.com"},{"label":"Experience","value":"6+ years in web development"},{"label":"Role","value":"Senior Backend Software Engineer"},{"label":"Date of Birth","value":"27/10/1996"},{"label":"Address","value":"My Chanh Commune, Phu My District, Binh Dinh Province"}]}}'::jsonb
where id = 1 and content = '{}'::jsonb;
-- +goose StatementEnd

-- +goose StatementBegin
update public.site_settings set
    site_title = 'QDJr Blog',
    site_description = 'This is a small blog of Nguyen Quoc Dai',
    contact_email = 'quocdaijr@gmail.com',
    social_links = '{"mail":"mailto:quocdaijr@gmail.com","telegram":"https://t.me/quocdaijr","signal":"https://signal.me/#eu/U1eIvUn29-2TB8fu6lTq-nxX0qOa3LNUCoyywDx_WVvEdYF7C5NK15eKfieS3TDe","discord":"https://discord.com/users/quocdaijr","github":"https://github.com/quocdaijr","facebook":"https://facebook.com/quocdaijr","linkedin":"https://www.linkedin.com/in/quocdaijr"}'::jsonb
where id = 1 and social_links = '{}'::jsonb;
-- +goose StatementEnd

insert into public.categories (slug, name) values ('tools', 'Tools') on conflict (slug) do nothing;
insert into public.categories (slug, name) values ('meta', 'Meta') on conflict (slug) do nothing;
insert into public.tags (slug, name) values ('claude-code', 'Claude Code') on conflict (slug) do nothing;
insert into public.tags (slug, name) values ('ai', 'Ai') on conflict (slug) do nothing;
insert into public.tags (slug, name) values ('workflow', 'Workflow') on conflict (slug) do nothing;
insert into public.tags (slug, name) values ('vietnamese', 'Vietnamese') on conflict (slug) do nothing;
insert into public.tags (slug, name) values ('nuxt', 'Nuxt') on conflict (slug) do nothing;
insert into public.tags (slug, name) values ('markdown', 'Markdown') on conflict (slug) do nothing;

-- +goose StatementBegin
insert into public.posts (slug, title, excerpt, content, status, published_at)
values ('claude-code-tro-ly-lap-trinh', 'Claude Code — Trợ lý lập trình trong terminal mà mình đang dùng', 'Giới thiệu nhanh về Claude Code của Anthropic, cách mình dùng nó hằng ngày, và vài mẹo để đỡ tốn token.', $md$> 🤖 *Heads up: bài này được AI generate ra để mình nghịch thử feature blog mới thôi nha.
> Đọc mà thấy hay hay thì vui, thấy sai sai thì bro cứ skip, đừng trách mình 😄*

Trong mấy tháng gần đây, công cụ AI mình dùng nhiều nhất khi code không phải là một IDE nào cả,
mà là **Claude Code** — một CLI do Anthropic phát triển, chạy thẳng trong terminal. Bài này
mình viết lại nhanh những gì mình thấy hữu ích để ai chưa dùng có thể hình dung.

## Claude Code là gì?

Claude Code là một **AI agent chạy trong terminal**, có quyền đọc/ghi file, chạy lệnh shell,
dùng git, gọi API và tự tổ chức công việc qua todo list. Khác với mấy plugin IDE kiểu Copilot
— cái đó chủ yếu gợi ý từng dòng — Claude Code thực sự *thực hiện task* giúp mình:

- "Đọc thư mục `pages/`, tìm chỗ nào còn gọi API cũ và viết lại bằng `@nuxt/content`" → nó làm.
- "Chạy `npm run build`, nếu lỗi thì đọc log và tự fix" → nó làm.
- "Tạo PR với commit message rõ ràng" → cũng làm luôn.

Model mặc định là **Claude Sonnet / Opus** (Anthropic). Bạn có thể bật/tắt extended thinking,
đổi model giữa phiên, và chạy song song nhiều subagent độc lập cho các task lớn.

## Mình dùng vào việc gì?

Ba nhóm chính:

**1. Refactor codebase lớn.** Ví dụ vụ mình vừa làm: gỡ blog cũ phụ thuộc API đã tắt,
chuyển sang markdown với `@nuxt/content`, mà vẫn giữ lại code cũ ở `/legacy-blogs`. Claude Code
đọc toàn bộ repo, lên kế hoạch, di chuyển file bằng `git mv` (để giữ history), viết page mới,
chạy dev server verify, rồi báo cáo lại. Mình chỉ review.

**2. Debug production.** Paste log lỗi vào, nó sẽ grep codebase, đọc các file liên quan,
đoán root cause và đề xuất fix. Không cần mình bật 5 tab StackOverflow.

**3. Viết tài liệu / commit message.** Đặc biệt hợp với repo cá nhân bị "no-commit-message"
chronic như của mình.

## Vài mẹo tiết kiệm

Claude Code tính tiền theo token, nên xài bừa dễ hết budget nhanh. Mấy thứ mình thấy hiệu quả:

```bash
# Dùng subagent Explore để khảo sát code — nó gom context thay vì đổ hết vào main session
/agent Explore "tìm tất cả chỗ gọi API /posts trong repo"

# Dùng Plan mode cho task lớn — đọc trước, không sửa cho tới khi bạn duyệt plan
# (gõ /plan hoặc Shift+Tab để vào)

# Bật compaction khi session dài — tự nén context cũ
# Config ở ~/.claude/settings.json
```

## So với các tool khác?

Mình từng thử:

- **Cursor / Windsurf**: fork VS Code, UX đẹp, nhưng vẫn là editor-first — context window không
  linh hoạt bằng.
- **GitHub Copilot**: tốt cho autocomplete, kém cho task đa bước.
- **Aider**: cùng triết lý CLI agent nhưng ít công cụ tích hợp hơn và cộng đồng nhỏ hơn.

Claude Code thắng khi task cần **nhiều bước phụ thuộc nhau** và cần chạy lệnh thật sự (test,
build, git, curl...). Nếu chỉ gõ hàm ngắn thì Copilot vẫn nhanh hơn.

## Tổng kết

Mình đang dùng Claude Code làm trợ lý mặc định cho mọi task dev trên máy cá nhân. Nó không
thay thế kỹ năng code, nhưng cắt được kha khá công việc lặp lại và giúp mình giữ flow khi
chuyển ngữ cảnh.

Bài tiếp theo mình sẽ viết về cấu hình `~/.claude/settings.json` — hooks, permission, MCP
server — những thứ biến Claude Code từ "con bot phản hồi prompt" thành một phần thực sự của
workflow dev.$md$, 'published', '2026-04-19T08:30:00.000Z'::timestamptz)
on conflict (slug) do nothing;
-- +goose StatementEnd
-- +goose StatementBegin
insert into public.post_categories (post_id, category_id)
select p.id, c.id from public.posts p, public.categories c where p.slug = 'claude-code-tro-ly-lap-trinh' and c.slug = 'tools'
on conflict do nothing;
-- +goose StatementEnd
-- +goose StatementBegin
insert into public.post_tags (post_id, tag_id)
select p.id, t.id from public.posts p, public.tags t where p.slug = 'claude-code-tro-ly-lap-trinh' and t.slug = 'claude-code'
on conflict do nothing;
-- +goose StatementEnd
-- +goose StatementBegin
insert into public.post_tags (post_id, tag_id)
select p.id, t.id from public.posts p, public.tags t where p.slug = 'claude-code-tro-ly-lap-trinh' and t.slug = 'ai'
on conflict do nothing;
-- +goose StatementEnd
-- +goose StatementBegin
insert into public.post_tags (post_id, tag_id)
select p.id, t.id from public.posts p, public.tags t where p.slug = 'claude-code-tro-ly-lap-trinh' and t.slug = 'workflow'
on conflict do nothing;
-- +goose StatementEnd
-- +goose StatementBegin
insert into public.post_tags (post_id, tag_id)
select p.id, t.id from public.posts p, public.tags t where p.slug = 'claude-code-tro-ly-lap-trinh' and t.slug = 'vietnamese'
on conflict do nothing;
-- +goose StatementEnd

-- +goose StatementBegin
insert into public.posts (slug, title, excerpt, content, status, published_at)
values ('hello-world', 'Hello, World — The Blog Is Back', 'Blog sống lại sau một thời gian nghỉ vì backend bị tắt. Bài này kể nhanh câu chuyện tại sao mình chọn Markdown + Git thay vì host lại API hay ôm một headless CMS.', $md$> 🤖 *Heads up: bài này được AI generate ra để mình nghịch thử feature blog mới thôi nha.
> Đọc mà thấy hay hay thì vui, thấy sai sai thì bro cứ skip, đừng trách mình 😄*

Welcome back. Blog này trước đây chạy trên một backend REST riêng
(`api.qdjr.me/v1`) do chính mình viết và host. Nó ổn trong mấy năm đầu, cho tới
khi mình quyết định tắt server để tiết kiệm chi phí, và... quên mất là blog vẫn
đang phụ thuộc vào nó. Kết quả là ai vào `/blog` cũng thấy một câu buồn bã:
*"Will be back soon 🥲"*.

## Các lựa chọn đã cân nhắc

Khi ngồi xuống fix, mình có ba hướng:

1. **Host lại backend**: Viết lại API, deploy lên VPS nào đó. Giá rẻ nhưng lại
   phải maintain.
2. **Dùng headless CMS free tier**: Sanity, Strapi Cloud, Contentful, Tina...
   Nhìn xa xa thì free, nhưng hầu hết 2024–2025 đã siết lại. Supabase thì
   auto-pause sau 7 ngày không hoạt động. PlanetScale đã bỏ free tier từ tháng
   4/2024.
3. **Bỏ hẳn backend, lưu bài bằng Markdown trong Git**: [`@nuxt/content`](https://content.nuxt.com/)
   làm sẵn module này cho Nuxt.

## Vì sao chọn Markdown

Một câu hỏi đơn giản: *"Nếu dịch vụ mình chọn đóng cửa sau 2 năm, mình mất gì?"*

- Backend tự host: mất thời gian maintain, uptime, backup.
- Headless CMS: mất luôn dữ liệu nếu không export kịp, hoặc bị ép lên gói trả phí.
- Markdown + Git: **không mất gì cả**. File `.md` vẫn nằm đó, có thể deploy đi
  bất cứ đâu.

Ngoài ra:

- Bài viết được version hoá cùng code — `git log` ra lịch sử chỉnh sửa.
- Không cần auth, không cold start, không rate limit.
- Build ra static site, cache tốt, chạy nhanh.

Cái duy nhất hy sinh là **UI để viết bài**. Mình phải soạn trong editor
(VS Code, nvim, ...) rồi `git commit`. Với người làm dev solo thì không vấn đề,
thậm chí còn nhanh hơn.

## Code sample

Fetch danh sách bài viết giờ đơn giản thành một query tới local collection:

```ts
// Fetching posts is just a query against the local collection.
const { data: posts } = await useAsyncData('blog-list', () =>
  queryCollection('blog')
    .where('draft', '=', false)
    .order('publishedAt', 'DESC')
    .all()
)
```

Không có fetch network, không có try/catch cho API timeout, không có CORS —
build time đã resolve xong.

## Cấu trúc sau khi migrate

```
content/
  blog/
    hello-world.md
    claude-code-tro-ly-lap-trinh.md
pages/
  blog/
    index.vue          # list
    [slug].vue         # detail
    tag/[tag].vue
    category/[category].vue
  legacy-blogs/        # đóng băng code cũ, không động vào
```

Route `/legacy-blogs/*` vẫn còn đó để giữ link cũ, dù data đã chết — coi như
di tích.

## Kết

Nếu bạn đang cân nhắc làm blog cá nhân / portfolio và không có nhu cầu UI CMS
cho content team, thì markdown + `@nuxt/content` (hoặc Astro Content
Collections nếu dùng Astro) là lựa chọn gần như không có downside. Đừng phức
tạp hoá sớm.

More posts coming soon.$md$, 'published', '2026-04-19T02:00:00.000Z'::timestamptz)
on conflict (slug) do nothing;
-- +goose StatementEnd
-- +goose StatementBegin
insert into public.post_categories (post_id, category_id)
select p.id, c.id from public.posts p, public.categories c where p.slug = 'hello-world' and c.slug = 'meta'
on conflict do nothing;
-- +goose StatementEnd
-- +goose StatementBegin
insert into public.post_tags (post_id, tag_id)
select p.id, t.id from public.posts p, public.tags t where p.slug = 'hello-world' and t.slug = 'nuxt'
on conflict do nothing;
-- +goose StatementEnd
-- +goose StatementBegin
insert into public.post_tags (post_id, tag_id)
select p.id, t.id from public.posts p, public.tags t where p.slug = 'hello-world' and t.slug = 'markdown'
on conflict do nothing;
-- +goose StatementEnd
-- +goose StatementBegin
insert into public.post_tags (post_id, tag_id)
select p.id, t.id from public.posts p, public.tags t where p.slug = 'hello-world' and t.slug = 'workflow'
on conflict do nothing;
-- +goose StatementEnd
-- +goose StatementBegin
insert into public.post_tags (post_id, tag_id)
select p.id, t.id from public.posts p, public.tags t where p.slug = 'hello-world' and t.slug = 'vietnamese'
on conflict do nothing;
-- +goose StatementEnd
