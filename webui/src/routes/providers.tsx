import { useState, useEffect } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { 
  Table, 
  TableBody, 
  TableCell, 
  TableHead, 
  TableHeader, 
  TableRow 
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Tooltip,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import Loading from "@/components/loading";
import {
  getProviders,
  createProvider,
  updateProvider,
  deleteProvider,
  getProviderTemplates,
  getProviderModels,
  getAllProvidersHealth,
  batchDeleteProviders,
  validateProviderConfig,
  exportConfig,
  importConfig,
  type ImportConfigData
} from "@/lib/api";
import type { Provider, ProviderTemplate, ProviderModel, ProviderHealth } from "@/lib/api";
import { Checkbox } from "@/components/ui/checkbox";

// 定义表单验证模式
const formSchema = z.object({
  name: z.string().min(1, { message: "提供商名称不能为空" }),
  type: z.string().min(1, { message: "提供商类型不能为空" }),
  config: z.string().min(1, { message: "配置不能为空" }),
  console: z.string().optional(),
});

export default function ProvidersPage() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [providerTemplates, setProviderTemplates] = useState<ProviderTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [editingProvider, setEditingProvider] = useState<Provider | null>(null);
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const [modelsOpen, setModelsOpen] = useState(false);
  const [modelsOpenId, setModelsOpenId] = useState<number | null>(null);
  const [providerModels, setProviderModels] = useState<ProviderModel[]>([]);
  const [filteredProviderModels, setFilteredProviderModels] = useState<ProviderModel[]>([]);
  const [modelsLoading, setModelsLoading] = useState(false);
  
  // 健康检查相关
  const [providersHealth, setProvidersHealth] = useState<Map<number, ProviderHealth>>(new Map());
  
  // 批量删除相关
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);
  
  // 配置验证相关
  const [validating, setValidating] = useState(false);
  const [validationResult, setValidationResult] = useState<string | null>(null);
  
  // 导入配置相关
  const [importing, setImporting] = useState(false);
  
  // 初始化表单
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: "",
      type: "",
      config: "",
      console: "",
    },
  });

  useEffect(() => {
    fetchProviders();
    fetchProviderTemplates();
    fetchProvidersHealth();
    
    // 每分钟刷新一次健康状态
    const interval = setInterval(fetchProvidersHealth, 60000);
    return () => clearInterval(interval);
  }, []);

  const fetchProviders = async () => {
    try {
      setLoading(true);
      const data = await getProviders();
      setProviders(data);
    } catch (err) {
      setError("获取提供商列表失败");
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const fetchProviderTemplates = async () => {
    try {
      const data = await getProviderTemplates();
      setProviderTemplates(data);
    } catch (err) {
      console.error("获取提供商模板失败", err);
    }
  };

  const fetchProvidersHealth = async () => {
    try {
      const data = await getAllProvidersHealth();
      const healthMap = new Map<number, ProviderHealth>();
      data.forEach(health => {
        healthMap.set(health.provider_id, health);
      });
      setProvidersHealth(healthMap);
    } catch (err) {
      console.error("获取提供商健康状态失败", err);
    }
  };

  const fetchProviderModels = async (providerId: number) => {
    try {
      setModelsLoading(true);
      const data = await getProviderModels(providerId);
      setProviderModels(data);
      setFilteredProviderModels(data);
    } catch (err) {
      console.error("获取提供商模型失败", err);
    } finally {
      setModelsLoading(false);
    }
  };

  const openModelsDialog = async (providerId: number) => {
    setModelsOpen(true);
    setModelsOpenId(providerId);
    await fetchProviderModels(providerId);
  };

  const copyModelName = (modelName: string) => {
    navigator.clipboard.writeText(modelName);
  };

  const copyAllModels = () => {
    const allModelNames = filteredProviderModels.map(m => m.id).join('\n');
    navigator.clipboard.writeText(allModelNames);
    alert(`已复制 ${filteredProviderModels.length} 个模型名称到剪贴板`);
  };

  const handleExport = () => {
    const downloadUrl = exportConfig();
    window.location.href = downloadUrl;
  };

  const handleImport = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;

    setImporting(true);
    try {
      const text = await file.text();
      const config: ImportConfigData = JSON.parse(text);
      
      const result = await importConfig(config);
      alert(`导入成功! 共导入 ${result.imported_count} 项配置`);
      fetchProviders();
      fetchProvidersHealth();
    } catch (err) {
      alert(`导入失败: ${err instanceof Error ? err.message : '未知错误'}`);
      console.error(err);
    } finally {
      setImporting(false);
      // 清空文件输入
      event.target.value = '';
    }
  };

  const handleCreate = async (values: z.infer<typeof formSchema>) => {
    // 先验证配置
    setValidating(true);
    setValidationResult(null);
    try {
      const validation = await validateProviderConfig({
        name: values.name,
        type: values.type,
        config: values.config,
        console: values.console || ""
      });
      
      if (!validation.valid) {
        setValidationResult(`配置验证失败: ${validation.error_message}`);
        setValidating(false);
        return;
      }
      
      setValidationResult(`配置验证成功! 响应时间: ${validation.response_time_ms}ms`);
      
      // 配置验证成功后创建
      await createProvider({
        name: values.name,
        type: values.type,
        config: values.config,
        console: values.console || ""
      });
      setOpen(false);
      form.reset({ name: "", type: "", config: "", console: "" });
      setValidationResult(null);
      fetchProviders();
      fetchProvidersHealth();
    } catch (err) {
      setError("创建提供商失败");
      console.error(err);
    } finally {
      setValidating(false);
    }
  };

  const handleUpdate = async (values: z.infer<typeof formSchema>) => {
    if (!editingProvider) return;
    try {
      await updateProvider(editingProvider.ID, {
        name: values.name,
        type: values.type,
        config: values.config,
        console: values.console || ""
      });
      setOpen(false);
      setEditingProvider(null);
      form.reset({ name: "", type: "", config: "", console: "" });
      fetchProviders();
    } catch (err) {
      setError("更新提供商失败");
      console.error(err);
    }
  };

  const handleDelete = async () => {
    if (!deleteId) return;
    try {
      await deleteProvider(deleteId);
      setDeleteId(null);
      fetchProviders();
      fetchProvidersHealth();
    } catch (err) {
      setError("删除提供商失败");
      console.error(err);
    }
  };

  const handleBatchDelete = async () => {
    if (selectedIds.size === 0) return;
    try {
      await batchDeleteProviders(Array.from(selectedIds));
      setSelectedIds(new Set());
      setBatchDeleteOpen(false);
      fetchProviders();
      fetchProvidersHealth();
    } catch (err) {
      setError("批量删除提供商失败");
      console.error(err);
    }
  };

  const toggleSelectAll = () => {
    if (selectedIds.size === providers.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(providers.map(p => p.ID)));
    }
  };

  const toggleSelect = (id: number) => {
    const newSelected = new Set(selectedIds);
    if (newSelected.has(id)) {
      newSelected.delete(id);
    } else {
      newSelected.add(id);
    }
    setSelectedIds(newSelected);
  };

  const openEditDialog = (provider: Provider) => {
    setEditingProvider(provider);
    form.reset({
      name: provider.Name,
      type: provider.Type,
      config: provider.Config,
      console: provider.Console || "",
    });
    setOpen(true);
  };

  const openCreateDialog = () => {
    setEditingProvider(null);
    form.reset({ name: "", type: "", config: "", console: "" });
    setOpen(true);
  };

  const openDeleteDialog = (id: number) => {
    setDeleteId(id);
  };

  if (loading) return <Loading message="加载提供商列表" />;
  if (error) return <div className="text-red-500">{error}</div>;

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-4">
        <h2 className="text-2xl font-bold">提供商管理</h2>
        <div className="flex gap-2">
          {selectedIds.size > 0 && (
            <Button
              variant="destructive"
              onClick={() => setBatchDeleteOpen(true)}
              className="w-full sm:w-auto"
            >
              批量删除 ({selectedIds.size})
            </Button>
          )}
          <Button onClick={handleExport} variant="outline" className="w-full sm:w-auto">
            导出配置
          </Button>
          <label htmlFor="import-config">
            <Button
              variant="outline"
              className="w-full sm:w-auto cursor-pointer"
              disabled={importing}
              onClick={() => document.getElementById('import-config')?.click()}
            >
              {importing ? '导入中...' : '导入配置'}
            </Button>
          </label>
          <input
            id="import-config"
            type="file"
            accept=".json"
            onChange={handleImport}
            style={{ display: 'none' }}
          />
          <Button onClick={openCreateDialog} className="w-full sm:w-auto">添加提供商</Button>
        </div>
      </div>
      
      {/* 桌面端表格 */}
      <div className="border rounded-lg hidden sm:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-12">
                <Checkbox
                  checked={selectedIds.size === providers.length && providers.length > 0}
                  onCheckedChange={toggleSelectAll}
                />
              </TableHead>
              <TableHead>ID</TableHead>
              <TableHead>名称</TableHead>
              <TableHead>类型</TableHead>
              <TableHead>健康状态</TableHead>
              <TableHead>配置</TableHead>
              <TableHead>控制台</TableHead>
              <TableHead>操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {providers.map((provider) => {
              const health = providersHealth.get(provider.ID);
              return (
              <TableRow key={provider.ID}>
                <TableCell>
                  <Checkbox
                    checked={selectedIds.has(provider.ID)}
                    onCheckedChange={() => toggleSelect(provider.ID)}
                  />
                </TableCell>
                <TableCell>{provider.ID}</TableCell>
                <TableCell>{provider.Name}</TableCell>
                <TableCell>{provider.Type}</TableCell>
                <TableCell>
                  {health ? (
                    <div className="flex items-center gap-2">
                      <span className={`px-2 py-1 rounded text-xs ${
                        health.status === 'healthy' ? 'bg-green-100 text-green-800' :
                        health.status === 'degraded' ? 'bg-yellow-100 text-yellow-800' :
                        'bg-red-100 text-red-800'
                      }`}>
                        {health.status}
                      </span>
                      <span className="text-xs text-gray-500">
                        {health.response_time_ms}ms
                      </span>
                    </div>
                  ) : (
                    <span className="text-gray-400">未知</span>
                  )}
                </TableCell>
                <TableCell>
                  <pre className="text-xs overflow-hidden max-w-md truncate">
                    {provider.Config}
                  </pre>
                </TableCell>
                <TableCell>
                  {provider.Console ? (
                    <Button 
                      title={provider.Console}
                      variant="outline" 
                      size="sm" 
                      onClick={() => window.open(provider.Console, '_blank')}
                    >
                      前往
                    </Button>
                  ) : (
                    "暂未设置"
                  )}
                </TableCell>
                <TableCell className="space-x-2">
                  <Button
                    variant="outline" 
                    size="sm" 
                    onClick={() => openEditDialog(provider)}
                  >
                    编辑
                  </Button>
                  <Button
                    variant="outline" 
                    size="sm" 
                    onClick={() => openModelsDialog(provider.ID)}
                  >
                    模型列表
                  </Button>
                  <AlertDialog>
                    <AlertDialogTrigger asChild>
                      <Button 
                        variant="destructive" 
                        size="sm" 
                        onClick={() => openDeleteDialog(provider.ID)}
                      >
                        删除
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>确定要删除这个提供商吗？</AlertDialogTitle>
                        <AlertDialogDescription>
                          此操作无法撤销。这将永久删除该提供商。
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel onClick={() => setDeleteId(null)}>取消</AlertDialogCancel>
                        <AlertDialogAction onClick={handleDelete}>确认删除</AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                </TableCell>
              </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>
      
      {/* 移动端卡片布局 */}
      <div className="sm:hidden space-y-4">
        {providers.map((provider) => (
          <div key={provider.ID} className="border rounded-lg p-4 space-y-3">
            <div className="flex justify-between items-start">
              <div>
                <h3 className="font-bold text-lg">{provider.Name}</h3>
                <p className="text-sm text-gray-500">ID: {provider.ID}</p>
                <p className="text-sm text-gray-500">类型: {provider.Type}</p>
                {provider.Console && (
                  <p className="text-sm">
                    <Button 
                      variant="link" 
                      size="sm" 
                      onClick={() => window.open(provider.Console, '_blank')}
                      className="p-0 h-auto"
                    >
                      前往
                    </Button>
                  </p>
                )}
              </div>
              <div className="flex space-x-2">
                <Button 
                  variant="outline" 
                  size="sm" 
                  onClick={() => openEditDialog(provider)}
                >
                  编辑
                </Button>
                <Button
                  variant="outline" 
                  size="sm" 
                  onClick={() => openModelsDialog(provider.ID)}
                >
                  模型列表
                </Button>
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button 
                      variant="destructive" 
                      size="sm" 
                      onClick={() => openDeleteDialog(provider.ID)}
                    >
                      删除
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>确定要删除这个提供商吗？</AlertDialogTitle>
                      <AlertDialogDescription>
                        此操作无法撤销。这将永久删除该提供商。
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel onClick={() => setDeleteId(null)}>取消</AlertDialogCancel>
                      <AlertDialogAction onClick={handleDelete}>确认删除</AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </div>
            </div>
          </div>
        ))}
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingProvider ? "编辑提供商" : "添加提供商"}
            </DialogTitle>
            <DialogDescription>
              {editingProvider 
                ? "修改提供商信息" 
                : "添加一个新的提供商"}
            </DialogDescription>
          </DialogHeader>
          
          <Form {...form}>
            <form onSubmit={form.handleSubmit(editingProvider ? handleUpdate : handleCreate)} className="space-y-4 min-w-0">
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>名称</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              
              <FormField
                control={form.control}
                name="type"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>类型</FormLabel>
                    <FormControl>
                      <select 
                        {...field} 
                        className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                        onChange={(e) => {
                          field.onChange(e);
                          // When type changes, populate config with template if available
                          const selectedTemplate = providerTemplates.find(t => t.type === e.target.value);
                          if (selectedTemplate) {
                            form.setValue("config", selectedTemplate.template);
                          }
                        }}
                      >
                        <option value="">请选择提供商类型</option>
                        {providerTemplates.map((template) => (
                          <option key={template.type} value={template.type}>
                            {template.type}
                          </option>
                        ))}
                      </select>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              
              <FormField
                control={form.control}
                name="config"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>配置</FormLabel>
                    <FormControl>
                      <Textarea 
                        {...field} 
                        className="resize-none whitespace-pre overflow-x-auto" 
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              
              <FormField
                control={form.control}
                name="console"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>控制台地址</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder="https://example.com/console" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              
              {validationResult && (
                <div className={`p-3 rounded ${
                  validationResult.includes('失败')
                    ? 'bg-red-50 text-red-800'
                    : 'bg-green-50 text-green-800'
                }`}>
                  {validationResult}
                </div>
              )}
              
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                  取消
                </Button>
                <Button type="submit" disabled={validating}>
                  {validating ? "验证中..." : (editingProvider ? "更新" : "创建")}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>

      {/* 模型列表对话框 */}
      <Dialog open={modelsOpen} onOpenChange={setModelsOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{providers.find(v => v.ID === modelsOpenId)?.Name}模型列表</DialogTitle>
            <DialogDescription>
              当前提供商的所有可用模型
            </DialogDescription>
          </DialogHeader>
          
          {/* 搜索框和复制所有按钮 */}
          {!modelsLoading && providerModels.length > 0 && (
            <div className="mb-4 flex gap-2">
              <Input
                placeholder="搜索模型 ID"
                onChange={(e) => {
                  const searchTerm = e.target.value.toLowerCase();
                  if (searchTerm === '') {
                    setFilteredProviderModels(providerModels);
                  } else {
                    const filteredModels = providerModels.filter(model =>
                      model.id.toLowerCase().includes(searchTerm)
                    );
                    setFilteredProviderModels(filteredModels);
                  }
                }}
                className="flex-1"
              />
              <Button
                variant="outline"
                onClick={copyAllModels}
              >
                复制所有 ({filteredProviderModels.length})
              </Button>
            </div>
          )}
          
          {modelsLoading ? (
            <Loading message="加载模型列表" />
          ) : (
            <div className="max-h-96 overflow-y-auto">
              {filteredProviderModels.length === 0 ? (
                <div className="text-center text-gray-500 py-8">
                  {providerModels.length === 0 ? '暂无模型数据' : '未找到匹配的模型'}
                </div>
              ) : (
                <div className="space-y-2">
                  {filteredProviderModels.map((model,index) => (
                    <div 
                      key={index} 
                      className="flex items-center justify-between p-3 border rounded-lg"
                    >
                      <div className="flex-1">
                        <div className="font-medium">{model.id}</div>
                      </div>
                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => copyModelName(model.id)}
                              className="ml-2"
                            >
                              复制
                            </Button>
                          </TooltipTrigger>
                        </Tooltip>
                      </TooltipProvider>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
          
          <DialogFooter>
            <Button onClick={() => setModelsOpen(false)}>关闭</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 批量删除确认对话框 */}
      <AlertDialog open={batchDeleteOpen} onOpenChange={setBatchDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确定要批量删除这些提供商吗？</AlertDialogTitle>
            <AlertDialogDescription>
              您将删除 {selectedIds.size} 个提供商，此操作无法撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setBatchDeleteOpen(false)}>取消</AlertDialogCancel>
            <AlertDialogAction onClick={handleBatchDelete}>确认删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}