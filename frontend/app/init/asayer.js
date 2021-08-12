import Asayer from '@openreplay/tracker';
import trackerAssist from '@openreplay/tracker-assist';
// import Fetch from '@openreplay/tracker-fetch';
// import Redux from '@openreplay/tracker-redux';
// import Profiler from '@openreplay/tracker-profiler';

// import { UPDATE as U_JWT, DELETE as D_JWT } from 'Duck/jwt'
// import { LOGIN, RESET_PASSWORD, REQUEST_RESET_PASSWORD, UPDATE_PASSWORD } from 'Duck/user';

const a = new Asayer({
	projectKey: window.ENV.TRACKER_PROJECT_ID,	
	ingestPoint: window.ENV.TRACKER_SERVER_URL,
});

a.start();
a.use(trackerAssist())

export default a;