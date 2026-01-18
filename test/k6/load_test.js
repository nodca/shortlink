import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// 自定义指标
const errorRate = new Rate('errors');
const redirectDuration = new Trend('redirect_duration');
const createDuration = new Trend('create_duration');

// 测试配置
export const options = {
    // 阶段式加压
    stages: [
        { duration: '10s', target: 10 },   // 预热：10秒内加到10用户
        { duration: '30s', target: 50 },   // 加压：30秒内加到50用户
        { duration: '1m', target: 100 },   // 峰值：1分钟保持100用户
        { duration: '30s', target: 50 },   // 降压：30秒降到50用户
        { duration: '10s', target: 0 },    // 收尾：10秒降到0
    ],
    thresholds: {
        http_req_duration: ['p(95)<500'],  // 95%请求<500ms
        errors: ['rate<0.1'],              // 错误率<10%
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:9999';

// 存储测试过程中创建的短码
let createdCodes = [];
let authToken = null;
let testUserId = `k6_user_${Date.now()}_${__VU}`;

// 初始化：每个VU执行一次
export function setup() {
    console.log(`开始压力测试，目标: ${BASE_URL}`);

    // 先创建一些短链用于跳转测试
    const codes = [];
    for (let i = 0; i < 10; i++) {
        const res = http.post(`${BASE_URL}/api/v1/shortlinks`,
            JSON.stringify({ url: `https://example.com/setup-${Date.now()}-${i}` }),
            { headers: { 'Content-Type': 'application/json' } }
        );
        if (res.status === 200) {
            const body = JSON.parse(res.body);
            if (body.code) {
                codes.push(body.code);
            }
        }
    }
    console.log(`预创建了 ${codes.length} 个短链用于测试`);
    return { codes };
}

// 主测试函数
export default function(data) {
    const codes = data.codes || [];

    // 模拟真实流量分布
    const rand = Math.random();

    if (rand < 0.70) {
        // 70% 短链跳转（最高频操作）
        testRedirect(codes);
    } else if (rand < 0.85) {
        // 15% 创建短链
        testCreateShortlink();
    } else if (rand < 0.95) {
        // 10% 用户注册登录流程
        testAuthFlow();
    } else {
        // 5% 查看我的短链
        testMyShortlinks();
    }

    sleep(0.1); // 模拟用户思考时间
}

// 测试短链跳转
function testRedirect(codes) {
    group('短链跳转', function() {
        let code;
        if (codes.length > 0) {
            code = codes[Math.floor(Math.random() * codes.length)];
        } else {
            code = 'test'; // fallback
        }

        const start = Date.now();
        const res = http.get(`${BASE_URL}/${code}`, {
            redirects: 0, // 不跟随重定向，只测量302响应时间
        });
        redirectDuration.add(Date.now() - start);

        const success = check(res, {
            '跳转成功(302)或404': (r) => r.status === 302 || r.status === 404,
        });
        errorRate.add(!success);
    });
}

// 测试创建短链
function testCreateShortlink() {
    group('创建短链', function() {
        const url = `https://example.com/k6-test-${Date.now()}-${Math.random().toString(36).substring(7)}`;

        const start = Date.now();
        const res = http.post(`${BASE_URL}/api/v1/shortlinks`,
            JSON.stringify({ url: url }),
            { headers: { 'Content-Type': 'application/json' } }
        );
        createDuration.add(Date.now() - start);

        const success = check(res, {
            '创建成功(200)': (r) => r.status === 200,
            '返回短码': (r) => {
                try {
                    const body = JSON.parse(r.body);
                    return body.code && body.code.length > 0;
                } catch {
                    return false;
                }
            },
        });
        errorRate.add(!success);

        // 保存创建的短码供后续测试使用
        if (res.status === 200) {
            try {
                const body = JSON.parse(res.body);
                if (body.code) {
                    createdCodes.push(body.code);
                }
            } catch {}
        }
    });
}

// 测试用户认证流程
function testAuthFlow() {
    group('用户认证', function() {
        const username = `k6user_${__VU}_${__ITER}`;
        const password = 'testpassword123';

        // 注册
        const regRes = http.post(`${BASE_URL}/api/v1/register`,
            JSON.stringify({ username: username, password: password }),
            { headers: { 'Content-Type': 'application/json' } }
        );

        // 注册可能失败（用户已存在），这是正常的
        if (regRes.status === 201 || regRes.status === 409) {
            // 登录
            const loginRes = http.post(`${BASE_URL}/api/v1/login`,
                JSON.stringify({ username: username, password: password }),
                { headers: { 'Content-Type': 'application/json' } }
            );

            const success = check(loginRes, {
                '登录成功(200)': (r) => r.status === 200,
                '返回token': (r) => {
                    try {
                        const body = JSON.parse(r.body);
                        return body.token && body.token.length > 0;
                    } catch {
                        return false;
                    }
                },
            });
            errorRate.add(!success);

            if (loginRes.status === 200) {
                try {
                    authToken = JSON.parse(loginRes.body).token;
                } catch {}
            }
        }
    });
}

// 测试查看我的短链
function testMyShortlinks() {
    group('我的短链', function() {
        // 先确保有token
        if (!authToken) {
            testAuthFlow();
        }

        if (authToken) {
            const res = http.get(`${BASE_URL}/api/v1/users/mine`, {
                headers: {
                    'Authorization': `Bearer ${authToken}`,
                },
            });

            const success = check(res, {
                '获取成功(200)': (r) => r.status === 200,
            });
            errorRate.add(!success);
        }
    });
}

// 测试结束清理
export function teardown(data) {
    console.log('压力测试完成');
    console.log(`预创建短码数: ${data.codes ? data.codes.length : 0}`);
}
