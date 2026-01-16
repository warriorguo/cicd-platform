// Shared mock functions so tests can check calls on either axios or axios.create() instance
const mockGet = jest.fn(() => Promise.resolve({ data: {} }));
const mockPost = jest.fn(() => Promise.resolve({ data: {} }));
const mockPut = jest.fn(() => Promise.resolve({ data: {} }));
const mockDelete = jest.fn(() => Promise.resolve({ data: {} }));
const mockPatch = jest.fn(() => Promise.resolve({ data: {} }));

const mockAxiosInstance = {
  get: mockGet,
  post: mockPost,
  put: mockPut,
  delete: mockDelete,
  patch: mockPatch,
  interceptors: {
    request: {
      use: jest.fn(),
      eject: jest.fn(),
    },
    response: {
      use: jest.fn(),
      eject: jest.fn(),
    },
  },
  defaults: {
    headers: {
      common: {},
    },
  },
};

const axios = {
  create: jest.fn(() => mockAxiosInstance),
  get: mockGet,
  post: mockPost,
  put: mockPut,
  delete: mockDelete,
  patch: mockPatch,
  interceptors: {
    request: {
      use: jest.fn(),
      eject: jest.fn(),
    },
    response: {
      use: jest.fn(),
      eject: jest.fn(),
    },
  },
  defaults: {
    headers: {
      common: {},
    },
  },
};

export default axios;
