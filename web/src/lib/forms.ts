export type NodeFormValues = {
  name: string;
  protocol: string;
  server: string;
  serverPort: string;
  enabled: boolean;
  transportJSON: string;
  tlsJSON: string;
  rawPayloadJSON: string;
  credential: string;
};

export type SubscriptionFormValues = {
  name: string;
  url: string;
  enabled: boolean;
};

export type GroupFormValues = {
  name: string;
  filterRegex: string;
  description: string;
};

export type TunnelFormValues = {
  name: string;
  groupID: string;
  listenHost: string;
  username: string;
  password: string;
};

export type FormErrors<T extends string> = Partial<Record<T, string>>;

function isJSON(value: string) {
  if (!value.trim()) {
    return true;
  }
  JSON.parse(value);
  return true;
}

export function validateNodeForm(values: NodeFormValues) {
  const errors: FormErrors<keyof NodeFormValues> = {};
  if (!values.name.trim()) {
    errors.name = "节点名称不能为空";
  }
  if (!values.protocol.trim()) {
    errors.protocol = "协议不能为空";
  }
  if (!values.server.trim()) {
    errors.server = "服务器地址不能为空";
  }
  const serverPort = Number(values.serverPort);
  if (!Number.isInteger(serverPort) || serverPort <= 0 || serverPort > 65535) {
    errors.serverPort = "端口必须在 1-65535 之间";
  }
  if (!values.credential.trim()) {
    errors.credential = "认证信息不能为空";
  }

  try {
    isJSON(values.transportJSON);
  } catch {
    errors.transportJSON = "transport_json 必须是合法 JSON";
  }
  try {
    isJSON(values.tlsJSON);
  } catch {
    errors.tlsJSON = "tls_json 必须是合法 JSON";
  }
  try {
    isJSON(values.rawPayloadJSON);
  } catch {
    errors.rawPayloadJSON = "raw_payload_json 必须是合法 JSON";
  }

  return errors;
}

export function validateSubscriptionForm(values: SubscriptionFormValues) {
  const errors: FormErrors<keyof SubscriptionFormValues> = {};
  if (!values.name.trim()) {
    errors.name = "订阅名称不能为空";
  }
  try {
    new URL(values.url);
  } catch {
    errors.url = "订阅地址必须是合法 URL";
  }
  return errors;
}

export function validateGroupForm(values: GroupFormValues) {
  const errors: FormErrors<keyof GroupFormValues> = {};
  if (!values.name.trim()) {
    errors.name = "分组名称不能为空";
  }
  try {
    new RegExp(values.filterRegex);
  } catch {
    errors.filterRegex = "正则表达式非法";
  }
  return errors;
}

export function validateTunnelForm(values: TunnelFormValues) {
  const errors: FormErrors<keyof TunnelFormValues> = {};
  if (!values.name.trim()) {
    errors.name = "隧道名称不能为空";
  }
  if (!values.groupID.trim()) {
    errors.groupID = "必须选择一个分组";
  }
  if ((values.username.trim() && !values.password.trim()) || (!values.username.trim() && values.password.trim())) {
    errors.password = "用户名和密码必须同时填写";
  }
  return errors;
}

export function hasErrors<T extends string>(errors: FormErrors<T>) {
  return Object.keys(errors).length > 0;
}

