"use client"

import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import Loading from "@/components/loading";
import {
  getMetrics,
  getModelCounts,
  getDashboardStats,
  getAllProvidersHealth
} from "@/lib/api";
import type { MetricsData, ModelCount, DashboardStats, ProviderHealth } from "@/lib/api";
import { ChartPieDonutText } from "@/components/charts/pie-chart";
import { ModelRankingChart } from "@/components/charts/bar-chart";

// Animated counter component
const AnimatedCounter = ({ value, duration = 1000 }: { value: number; duration?: number }) => {
  const [count, setCount] = useState(0);

  useEffect(() => {
    let startTime: number | null = null;
    const animateCount = (timestamp: number) => {
      if (!startTime) startTime = timestamp;
      const progress = timestamp - startTime;
      const progressRatio = Math.min(progress / duration, 1);
      const currentValue = Math.floor(progressRatio * value);
      
      setCount(currentValue);
      
      if (progress < duration) {
        requestAnimationFrame(animateCount);
      }
    };
    
    requestAnimationFrame(animateCount);
  }, [value, duration]);

  return <div className="text-3xl font-bold">{count.toLocaleString()}</div>;
};

export default function Home() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeChart, setActiveChart] = useState<"distribution" | "ranking">("distribution");
  
  // Real data from APIs
  const [todayMetrics, setTodayMetrics] = useState<MetricsData>({ reqs: 0, tokens: 0 });
  const [totalMetrics, setTotalMetrics] = useState<MetricsData>({ reqs: 0, tokens: 0 });
  const [modelCounts, setModelCounts] = useState<ModelCount[]>([]);
  const [dashboardStats, setDashboardStats] = useState<DashboardStats | null>(null);
  const [providersHealth, setProvidersHealth] = useState<ProviderHealth[]>([]);

  useEffect(() => {
    Promise.all([
      fetchTodayMetrics(),
      fetchTotalMetrics(),
      fetchModelCounts(),
      fetchDashboardStats(),
      fetchProvidersHealth()
    ]);
  }, []);
  
  const fetchTodayMetrics = async () => {
    try {
      const data = await getMetrics(0);
      setTodayMetrics(data);
    } catch (err) {
      setError("获取今日指标失败");
      console.error(err);
    }
  };
  
  const fetchTotalMetrics = async () => {
    try {
      const data = await getMetrics(30); // Get last 30 days for "total" metrics
      setTotalMetrics(data);
    } catch (err) {
      setError("获取总计指标失败");
      console.error(err);
    }
  };
  
  const fetchModelCounts = async () => {
    try {
      const data = await getModelCounts();
      setModelCounts(data);
    } catch (err) {
      setError("获取模型调用统计失败");
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const fetchDashboardStats = async () => {
    try {
      const data = await getDashboardStats();
      setDashboardStats(data);
    } catch (err) {
      console.error("获取仪表板统计失败", err);
    }
  };

  const fetchProvidersHealth = async () => {
    try {
      const data = await getAllProvidersHealth();
      setProvidersHealth(data);
    } catch (err) {
      console.error("获取提供商健康状态失败", err);
    }
  };

  if (loading) return <Loading message="加载系统概览" />;
  if (error) return <div className="text-red-500">{error}</div>;

  return (
    <div className="space-y-6">
      {/* 系统状态概览 */}
      {dashboardStats && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          <Card>
            <CardHeader>
              <CardTitle>提供商总数</CardTitle>
              <CardDescription>系统中配置的提供商数量</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold">{dashboardStats.total_providers}</div>
              <p className="text-sm text-green-600 mt-2">
                健康: {dashboardStats.healthy_providers}
              </p>
            </CardContent>
          </Card>
          
          <Card>
            <CardHeader>
              <CardTitle>24h请求数</CardTitle>
              <CardDescription>最近24小时处理的请求</CardDescription>
            </CardHeader>
            <CardContent>
              <AnimatedCounter value={dashboardStats.total_requests_24h} />
              <p className="text-sm text-gray-500 mt-2">
                成功: {dashboardStats.success_requests_24h} | 失败: {dashboardStats.failed_requests_24h}
              </p>
            </CardContent>
          </Card>
          
          <Card>
            <CardHeader>
              <CardTitle>成功率</CardTitle>
              <CardDescription>最近24小时成功率</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold">
                {dashboardStats.total_requests_24h > 0
                  ? ((dashboardStats.success_requests_24h / dashboardStats.total_requests_24h) * 100).toFixed(2)
                  : 0}%
              </div>
            </CardContent>
          </Card>
          
          <Card>
            <CardHeader>
              <CardTitle>平均响应时间</CardTitle>
              <CardDescription>最近24小时平均响应</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold">{dashboardStats.avg_response_time_ms}ms</div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* 今日和本月统计 */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <Card>
          <CardHeader>
            <CardTitle>今日请求</CardTitle>
            <CardDescription>今日处理的请求总数</CardDescription>
          </CardHeader>
          <CardContent>
            <AnimatedCounter value={todayMetrics.reqs} />
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader>
            <CardTitle>今日Tokens</CardTitle>
            <CardDescription>今日处理的Tokens总数</CardDescription>
          </CardHeader>
          <CardContent>
            <AnimatedCounter value={todayMetrics.tokens} />
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader>
            <CardTitle>本月请求</CardTitle>
            <CardDescription>最近30天处理的请求总数</CardDescription>
          </CardHeader>
          <CardContent>
            <AnimatedCounter value={totalMetrics.reqs} />
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader>
            <CardTitle>本月Tokens</CardTitle>
            <CardDescription>最近30天处理的Tokens总数</CardDescription>
          </CardHeader>
          <CardContent>
            <AnimatedCounter value={totalMetrics.tokens} />
          </CardContent>
        </Card>
      </div>

      {/* 提供商健康状态 */}
      {providersHealth.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>提供商健康状态</CardTitle>
            <CardDescription>实时监控提供商运行状态</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {providersHealth.map((provider) => (
                <div key={provider.provider_id} className="border rounded-lg p-4">
                  <div className="flex justify-between items-start mb-2">
                    <h3 className="font-semibold">{provider.provider_name}</h3>
                    <span className={`px-2 py-1 rounded text-xs ${
                      provider.status === 'healthy' ? 'bg-green-100 text-green-800' :
                      provider.status === 'degraded' ? 'bg-yellow-100 text-yellow-800' :
                      'bg-red-100 text-red-800'
                    }`}>
                      {provider.status}
                    </span>
                  </div>
                  <div className="text-sm space-y-1">
                    <p className="text-gray-600">类型: {provider.provider_type}</p>
                    <p className="text-gray-600">响应时间: {provider.response_time_ms}ms</p>
                    <p className="text-gray-600">24h成功率: {provider.success_rate_24h.toFixed(2)}%</p>
                    <p className="text-gray-600">24h请求数: {provider.total_requests_24h}</p>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Top 5 模型统计 */}
      {dashboardStats && dashboardStats.top_models.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Top 5 热门模型</CardTitle>
            <CardDescription>最近24小时使用最多的模型</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {dashboardStats.top_models.map((model, index) => (
                <div key={index} className="flex items-center justify-between border-b pb-2">
                  <div className="flex-1">
                    <h4 className="font-medium">{model.model_name}</h4>
                    <p className="text-sm text-gray-500">
                      请求数: {model.request_count} | 成功率: {model.success_rate.toFixed(2)}%
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="text-sm font-medium">{model.total_tokens.toLocaleString()} tokens</p>
                    <p className="text-xs text-gray-500">平均响应: {model.avg_response_time_ms}ms</p>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Top 5 提供商统计 */}
      {dashboardStats && dashboardStats.top_providers.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Top 5 活跃提供商</CardTitle>
            <CardDescription>最近24小时请求最多的提供商</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {dashboardStats.top_providers.map((provider, index) => (
                <div key={index} className="flex items-center justify-between border-b pb-2">
                  <div className="flex-1">
                    <h4 className="font-medium">{provider.provider_name}</h4>
                    <p className="text-sm text-gray-500">
                      请求数: {provider.request_count} | 成功率: {provider.success_rate.toFixed(2)}%
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="text-sm font-medium">{provider.total_tokens.toLocaleString()} tokens</p>
                    <p className="text-xs text-gray-500">平均响应: {provider.avg_response_time_ms}ms</p>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
      
      <Card>
        <CardHeader>
          <CardTitle>模型数据分析</CardTitle>
          <CardDescription>模型调用统计分析</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex gap-2 mb-4">
            <Button 
              variant={activeChart === "distribution" ? "default" : "outline"} 
              onClick={() => setActiveChart("distribution")}
            >
              调用次数分布
            </Button>
            <Button 
              variant={activeChart === "ranking" ? "default" : "outline"} 
              onClick={() => setActiveChart("ranking")}
            >
              调用次数排行
            </Button>
          </div>
          <div className="mt-4">
            {activeChart === "distribution" ? <ChartPieDonutText data={modelCounts} /> : <ModelRankingChart data={modelCounts} />}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}