// Lists courses from the lessons service over Connect, from Node rather than
// the browser (@connectrpc/connect-node). Pass the HTTP version to compare:
// "1.1" works everywhere; "2" is h2c on a local http:// URL and ALPN h2 over
// https://.
//
//   QDJR_TOKEN=<access token> pnpm rpc:list [baseUrl] [1.1|2]
import { createClient, type Interceptor } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-node';

import { LessonsService } from '../gen/lessons/v1/lessons_pb';

async function main(): Promise<void> {
  const baseUrl = process.argv[2] ?? 'http://localhost:8080';
  const httpVersion = process.argv[3] === '2' ? '2' : '1.1';
  const token = process.env.QDJR_TOKEN;
  if (!token) throw new Error('set QDJR_TOKEN to an access token');

  const bearer: Interceptor = (next) => (req) => {
    req.header.set('Authorization', `Bearer ${token}`);
    return next(req);
  };
  const client = createClient(
    LessonsService,
    createConnectTransport({ baseUrl, httpVersion, interceptors: [bearer] })
  );

  const { courses, total } = await client.listCourses({ perPage: 100 });
  for (const c of courses) console.log(`${c.doneCount}/${c.lessonCount}  ${c.title}`);
  console.log(`${total} courses via Connect over HTTP/${httpVersion} from ${baseUrl}`);
}

main().catch((err: unknown) => {
  console.error(err instanceof Error ? err.message : err);
  process.exitCode = 1;
});
