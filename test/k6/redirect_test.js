import http from 'k6/http';
import { check } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// 自定义指标
const errorRate = new Rate('errors');
const redirectLatency = new Trend('redirect_latency', true);

// 测试配置 - 专注于跳转性能
export const options = {
    scenarios: {
        // 场景1：恒定并发用户
        constant_load: {
            executor: 'constant-vus',
            vus: 50,
            duration: '1m',
        },
        // 场景2：恒定请求速率 (RPS)
        // constant_rps: {
        //     executor: 'constant-arrival-rate',
        //     rate: 1000,           // 1000 RPS
        //     timeUnit: '1s',
        //     duration: '1m',
        //     preAllocatedVUs: 100,
        //     maxVUs: 200,
        // },
    },
    thresholds: {
        http_req_duration: ['p(50)<50', 'p(95)<200', 'p(99)<500'],
        errors: ['rate<0.01'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:9999';

// 预创建短链
export function setup() {
    const codes = [];
    for (let i = 0; i < 20; i++) {
        const res = http.post(`${BASE_URL}/api/v1/shortlinks`,
            JSON.stringify({ url: `https://example.com/redirect-test-${Date.now()}-${i}` }),
            { headers: { 'Content-Type': 'application/json' } }
        );
        if (res.status === 200) {
            try {
                const body = JSON.parse(res.body);
                if (body.code) codes.push(body.code);
            } catch {}
        }
    }
    console.log(`预创建 ${codes.length} 个短链`);
    return { codes };
}

export default function(data) {
    const codes = data.codes || [];
    if (codes.length === 0) {
        console.error('没有可用的短码');
        return;
    }

    // 随机选择一个短码
    const code = codes[Math.floor(Math.random() * codes.length)];

    const res = http.get(`${BASE_URL}/${code}`, {
        redirects: 0, // 不跟随重定向
        tags: { name: 'redirect' },
    });

    redirectLatency.add(res.timings.duration);

    const success = check(res, {
        '302 重定向': (r) => r.status === 302,
        'Location 头存在': (r) => r.headers['Location'] !== undefined,
        '响应时间 < 100ms': (r) => r.timings.duration < 100,
    });

    errorRate.add(!success);
}

export function teardown(data) {
    console.log('跳转性能测试完成');
}
