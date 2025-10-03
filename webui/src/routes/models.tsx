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
  getModels,
  createModel,
  updateModel,
  deleteModel,
  exportConfig,
  importConfig,
  type ImportConfigData
} from "@/lib/api";
import type { Model } from "@/lib/api";

// 定义表单验证模式
const formSchema = z.object({
  name: z.string().min(1, { message: "模型名称不能为空" }),
  remark: z.string(),
  max_retry: z.number().min(0, { message: "重试次数限制不能为负数" }),
  time_out: z.number().min(0, { message: "超时时间不能为负数" }),
});

export default function ModelsPage() {
  const [models, setModels] = useState<Model[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [editingModel, setEditingModel] = useState<Model | null>(null);
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const [importing, setImporting] = useState(false);
  
  // 初始化表单
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: "",
      remark: "",
      max_retry: 10,
      time_out: 60,
    },
  });

  useEffect(() => {
    console.log("Fetching models...");
    fetchModels();
  }, []);

  const fetchModels = async () => {
    try {
      setLoading(true);
      const data = await getModels();
      setModels(data);
    } catch (err) {
      setError("获取模型列表失败");
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async (values: z.infer<typeof formSchema>) => {
    try {
      await createModel(values);
      setOpen(false);
      form.reset({ name: "", remark: "", max_retry: 10, time_out: 60 });
      fetchModels();
    } catch (err) {
      setError("创建模型失败");
      console.error(err);
    }
  };

  const handleUpdate = async (values: z.infer<typeof formSchema>) => {
    if (!editingModel) return;
    try {
      await updateModel(editingModel.ID, values);
      setOpen(false);
      setEditingModel(null);
      form.reset({ name: "", remark: "", max_retry: 10, time_out: 60 });
      fetchModels();
    } catch (err) {
      setError("更新模型失败");
      console.error(err);
    }
  };

  const handleDelete = async () => {
    if (!deleteId) return;
    try {
      await deleteModel(deleteId);
      setDeleteId(null);
      fetchModels();
    } catch (err) {
      setError("删除模型失败");
      console.error(err);
    }
  };

  const openEditDialog = (model: Model) => {
    setEditingModel(model);
    form.reset({
      name: model.Name,
      remark: model.Remark,
      max_retry: model.MaxRetry,
      time_out: model.TimeOut,
    });
    setOpen(true);
  };

  const openCreateDialog = () => {
    setEditingModel(null);
    form.reset({ name: "", remark: "", max_retry: 10, time_out: 60 });
    setOpen(true);
  };

  const openDeleteDialog = (id: number) => {
    setDeleteId(id);
  };

  const handleExport = async () => {
    try {
      const token = localStorage.getItem("authToken");
      const response = await fetch('/api/config/export', {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });
      
      if (!response.ok) {
        throw new Error(`导出失败: ${response.status}`);
      }
      
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `llmio-config-${new Date().toISOString().slice(0,10)}.json`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      window.URL.revokeObjectURL(url);
    } catch (err) {
      alert(`导出失败: ${err instanceof Error ? err.message : '未知错误'}`);
      console.error(err);
    }
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
      fetchModels();
    } catch (err) {
      alert(`导入失败: ${err instanceof Error ? err.message : '未知错误'}`);
      console.error(err);
    } finally {
      setImporting(false);
      event.target.value = '';
    }
  };

  if (loading) return <Loading message="加载模型列表" />;
  if (error) return <div className="text-red-500">{error}</div>;

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-4">
        <h2 className="text-2xl font-bold">模型管理</h2>
        <div className="flex gap-2">
          <Button onClick={handleExport} variant="outline" className="w-full sm:w-auto">
            导出配置
          </Button>
          <label htmlFor="import-models-config">
            <Button
              variant="outline"
              className="w-full sm:w-auto cursor-pointer"
              disabled={importing}
              onClick={() => document.getElementById('import-models-config')?.click()}
            >
              {importing ? '导入中...' : '导入配置'}
            </Button>
          </label>
          <input
            id="import-models-config"
            type="file"
            accept=".json"
            onChange={handleImport}
            style={{ display: 'none' }}
          />
          <Button onClick={openCreateDialog} className="w-full sm:w-auto">添加模型</Button>
        </div>
      </div>
      
      {/* 桌面端表格 */}
      <div className="border rounded-lg hidden sm:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>名称</TableHead>
              <TableHead>备注</TableHead>
              <TableHead>重试次数限制</TableHead>
              <TableHead>超时时间(秒)</TableHead>
              <TableHead>操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {models.map((model) => (
              <TableRow key={model.ID}>
                <TableCell>{model.ID}</TableCell>
                <TableCell>{model.Name}</TableCell>
                <TableCell>{model.Remark}</TableCell>
                <TableCell>{model.MaxRetry}</TableCell>
                <TableCell>{model.TimeOut}</TableCell>
                <TableCell className="space-x-2">
                  <Button 
                    variant="outline" 
                    size="sm" 
                    onClick={() => openEditDialog(model)}
                  >
                    编辑
                  </Button>
                  <AlertDialog>
                    <AlertDialogTrigger asChild>
                      <Button 
                        variant="destructive" 
                        size="sm" 
                        onClick={() => openDeleteDialog(model.ID)}
                      >
                        删除
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>确定要删除这个模型吗？</AlertDialogTitle>
                        <AlertDialogDescription>
                          此操作无法撤销。这将永久删除该模型。
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
            ))}
          </TableBody>
        </Table>
      </div>
      
      {/* 移动端卡片布局 */}
      <div className="sm:hidden space-y-4">
        {models.map((model) => (
          <div key={model.ID} className="border rounded-lg p-4 space-y-3">
            <div className="flex justify-between items-start">
              <div>
                <h3 className="font-bold text-lg">{model.Name}</h3>
                <p className="text-sm text-gray-500">ID: {model.ID}</p>
              </div>
              <div className="flex space-x-2">
                <Button 
                  variant="outline" 
                  size="sm" 
                  onClick={() => openEditDialog(model)}
                >
                  编辑
                </Button>
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button 
                      variant="destructive" 
                      size="sm" 
                      onClick={() => openDeleteDialog(model.ID)}
                    >
                      删除
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>确定要删除这个模型吗？</AlertDialogTitle>
                      <AlertDialogDescription>
                        此操作无法撤销。这将永久删除该模型。
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
            <div>
              <p className="text-sm text-gray-600">{model.Remark}</p>
              <div className="grid grid-cols-2 gap-2 mt-2">
                <div className="text-sm">
                  <span className="text-gray-500">重试次数限制:</span>
                  <span className="ml-1">{model.MaxRetry}</span>
                </div>
                <div className="text-sm">
                  <span className="text-gray-500">超时时间:</span>
                  <span className="ml-1">{model.TimeOut}秒</span>
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingModel ? "编辑模型" : "添加模型"}
            </DialogTitle>
            <DialogDescription>
              {editingModel 
                ? "修改模型信息" 
                : "添加一个新的模型"}
            </DialogDescription>
          </DialogHeader>
          
          <Form {...form}>
            <form onSubmit={form.handleSubmit(editingModel ? handleUpdate : handleCreate)} className="space-y-4">
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
                name="remark"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>备注</FormLabel>
                    <FormControl>
                      <Textarea {...field} rows={3} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              
              <div className="grid grid-cols-2 gap-4">
                <FormField
                  control={form.control}
                  name="max_retry"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>重试次数限制</FormLabel>
                      <FormControl>
                        <Input 
                          type="number" 
                          {...field} 
                          onChange={e => field.onChange(+e.target.value)} 
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                
                <FormField
                  control={form.control}
                  name="time_out"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>超时时间(秒)</FormLabel>
                      <FormControl>
                        <Input 
                          type="number" 
                          {...field} 
                          onChange={e => field.onChange(+e.target.value)} 
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                  取消
                </Button>
                <Button type="submit">
                  {editingModel ? "更新" : "创建"}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>
    </div>
  );
}